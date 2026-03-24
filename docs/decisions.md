# Architecture Decision Records

Key architectural decisions made during the Pulse Pipeline project — what was chosen, what was considered, and why.

---

## ADR-001 · Kafka partition key = `customer_id`

**Decision:** Use `customer_id` as the Kafka message key for all events on `pulse.events.v1`.

**Context:** Kafka distributes messages across partitions using a hash of the message key. The choice of key determines which messages land on the same partition — and messages within a single partition are strictly ordered.

**Rationale:** For an event tracking pipeline, the most common query pattern is "give me all events for customer X in chronological order". By using `customer_id` as the key, all events for the same customer are guaranteed to land on the same partition and be consumed in order. This is more useful than random (round-robin) distribution which would scatter a customer's events across all partitions.

**Trade-off:** If a small number of customers generate a disproportionate volume of events, their partition becomes a hot spot. In production you'd add a suffix (e.g. `customer_id + sequence % N`) to spread hot customers across multiple partitions.

---

## ADR-002 · Kafka client = `segmentio/kafka-go` (not `confluent-kafka-go`)

**Decision:** Use `segmentio/kafka-go` as the Kafka client library.

**Context:** Two main Go Kafka clients exist: `confluent-kafka-go` (Confluent's official wrapper around `librdkafka`) and `segmentio/kafka-go` (pure Go implementation).

**Rationale:** `confluent-kafka-go` requires CGo, which means the build environment needs C headers and a C compiler. This complicates Docker multi-stage builds and cross-compilation. `segmentio/kafka-go` is pure Go — the same `go build` command works everywhere, and the Docker image is a clean static binary with no shared library dependencies. For a learning project where simplicity of the build pipeline matters, this was the right trade-off.

**Trade-off:** `librdkafka` is the most battle-tested Kafka client across all languages and typically offers more configuration knobs. For a learning project the difference is irrelevant.

---

## ADR-003 · Async Kafka producer (buffered channel + background goroutine)

**Decision:** The API service enqueues events to an in-memory channel and returns 202 immediately. A background goroutine drains the channel and writes to Kafka.

**Context:** The initial implementation called `kafka.WriteMessages()` synchronously inside the HTTP handler. Under the load test (20 workers × 100-event batches = up to 100 sequential Kafka writes per request), HTTP clients hit their write timeout before the handler returned — causing EOF errors on the client side despite all events reaching Kafka successfully.

**Rationale:** HTTP response time should not depend on Kafka write latency. By separating the two via a channel:
- The HTTP handler validates, enqueues, and returns in microseconds
- Kafka writes happen in the background, naturally batching whatever has accumulated in the channel
- `WriteTimeout` on the HTTP server is never triggered

**Trade-off:** Events in the channel are lost if the process crashes before they are written to Kafka. Acceptable for a learning project; in production you'd use a WAL or accept that the client must retry on 5xx.

**Capacity:** The channel is sized at 10,000 messages. If the background goroutine falls behind (Kafka unavailable), the channel fills and `Produce()` returns an error — the HTTP handler then returns 503.

---

## ADR-004 · MongoDB `$setOnInsert` upsert for idempotency

**Decision:** Each consumer write is `updateOne({ event_id: X }, { $setOnInsert: doc }, { upsert: true })` rather than a plain insert.

**Context:** Kafka consumer groups provide at-least-once delivery. If the consumer crashes after writing to MongoDB but before committing the offset, the same messages will be redelivered on restart.

**Rationale:** `$setOnInsert` only applies the update when the document does not already exist (i.e. when the upsert creates a new document). If `event_id` already exists, the filter matches an existing document and `$setOnInsert` is skipped — the document is **not** overwritten. This makes every write idempotent: sending the same event twice produces exactly one document.

**Why not `$set`?** `$set` would overwrite existing fields on duplicate delivery, which could corrupt data if the second delivery contained a different payload (e.g. due to a bug). `$setOnInsert` is strictly safer.

**Trade-off:** Upserts are slightly more expensive than plain inserts because MongoDB must check for an existing document first. The unique index on `event_id` makes this check a fast index lookup rather than a collection scan.

---

## ADR-005 · Commit Kafka offsets only after successful MongoDB write

**Decision:** `reader.CommitMessages()` is called only after `BulkWrite()` succeeds. If the write fails, offsets are not committed.

**Context:** This is the core reliability contract of the consumer: events must reach MongoDB or be retried. Committing offsets before writing would mean a crash between commit and write causes permanent data loss.

**Rationale:** By committing only after success, the worst case is duplicate delivery (at-least-once). Duplicates are handled by the idempotent upsert (ADR-004). The pipeline therefore achieves exactly-once storage semantics in MongoDB despite Kafka's at-least-once transport guarantee.

**Trade-off:** If MongoDB is unavailable for an extended period, consumer lag grows and the Kafka topic's retention window could expire, causing message loss. For production you'd set a dead-man alert on consumer lag (which we have in Cloud Monitoring) and ensure MongoDB availability SLAs exceed Kafka retention.

---

## ADR-006 · Dead Letter Queue after 3 retries

**Decision:** Events that fail to write after 3 consecutive attempts are produced to `pulse.events.dlq.v1` and the original offsets are committed.

**Context:** A permanently broken event (e.g. malformed after leaving the API) would block the consumer indefinitely if we never move past it — all subsequent events on that partition would be stuck.

**Rationale:** Three retries balances between transient failures (network blips, MongoDB restarts) and permanent failures (schema bugs). After three failures the event is moved to the DLQ for manual inspection, and the consumer continues processing. The DLQ has 30-day retention — long enough for investigation.

**Trade-off:** Three is an arbitrary number. In production you'd tune it based on observed transient failure rates and acceptable reprocessing latency.

---

## ADR-007 · Custom Prometheus registry (not the default global)

**Decision:** Both services register all metrics on a `prometheus.NewRegistry()` instance rather than the default `prometheus.DefaultRegisterer`.

**Context:** The default registry automatically includes Go runtime metrics (`go_*`) and process metrics (`process_*`). In tests, if multiple test cases try to register the same metric name on the default registry, they panic with "duplicate metrics collector registration".

**Rationale:** A custom registry gives full control over what is exposed. Metrics are explicitly registered, tests can create fresh instances without collision, and the `/metrics` endpoint returns only the metrics this service cares about. Go runtime and process metrics are added back explicitly via `collectors.NewGoCollector()` and `collectors.NewProcessCollector()`.

**Trade-off:** Slightly more boilerplate in `main.go`. Worth it for test isolation.

---

## ADR-008 · Flush buffer at 100 events OR 1 second (whichever comes first)

**Decision:** The consumer accumulates events in a buffer and flushes when the buffer hits 100 events or a 1-second ticker fires.

**Context:** MongoDB bulk writes are significantly more efficient than individual inserts (one round-trip vs. N round-trips). But waiting indefinitely for a full batch means low-traffic periods have high end-to-end latency.

**Rationale:** The dual condition (size OR time) is the standard pattern for batch sinks:
- At high throughput: batches fill quickly, frequent flushes, low latency
- At low throughput: the 1-second ticker ensures events are written within 1 second even if the batch never fills

**Trade-off:** 100 events and 1 second are tunable parameters (exposed via environment variables). In production you'd tune these based on observed throughput and acceptable write latency.

---

## ADR-009 · GKE over Cloud Run for the Consumer service

**Decision:** The Terraform/K8s configs deploy the Consumer on GKE, not Cloud Run.

**Context:** Cloud Run is Google's managed container platform — it handles scaling, zero-downtime deploys, and has no infrastructure to manage. For the API service it would be a natural fit.

**Rationale:** The Consumer maintains a persistent, stateful Kafka consumer group connection. Consumer group membership is tracked by the Kafka broker: when a consumer joins or leaves, the broker triggers a **rebalance** that reassigns partitions and pauses consumption during the transition. Cloud Run's scale-to-zero and ephemeral instance model would cause continuous rebalances, degrading throughput. GKE pods are long-lived and maintain stable consumer group membership.

**Trade-off:** GKE requires managing node pools and cluster upgrades. For the API service (stateless), Cloud Run would be simpler — but consistency between services (both on GKE) simplifies networking, IAM, and observability.

---

## ADR-010 · BigQuery partitioned by `DATE(timestamp)`, clustered by `customer_id + event_type`

**Decision:** The BigQuery `events` table uses time-based partitioning on `timestamp` and clustering on `(customer_id, event_type)`.

**Context:** BigQuery bills and performs based on bytes scanned. A full table scan over billions of events is expensive; partition and cluster pruning dramatically reduce this.

**Rationale:**
- **Partitioning by date:** The most common analytical queries are time-bounded ("last 7 days", "last month"). Partitioning by `DATE(timestamp)` means BigQuery only opens the relevant daily partitions — a query for "last 7 days" scans at most 7 partitions regardless of total table size.
- **Clustering by `customer_id, event_type`:** Within each partition, rows are physically sorted by these columns. A query like `WHERE customer_id = 'X' AND event_type = 'purchase'` scans only the relevant sorted block rather than the entire partition.

**Trade-off:** Clustering is only effective when queries filter on the clustered columns in order. A query filtering only on `event_type` benefits from clustering but not as much as one filtering on both columns.
