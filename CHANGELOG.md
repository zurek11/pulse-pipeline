# Changelog

All notable changes to Pulse Pipeline are documented in this file.

Format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Versioning Strategy

| Version | Meaning |
|---|---|
| `0.x.0` | Each project phase completes a new minor version |
| `0.x.z` | Bug fixes and non-breaking improvements within a phase |
| `1.0.0` | All 7 phases complete — project is interview-ready |

Planned releases: `v0.1.0` → `v0.2.0` → ... → `v0.6.0` → `v1.0.0`

---

## [Unreleased]

---

## [0.6.0] - 2026-03-24

### Phase 6: GCP Infrastructure 🏗️☁️

**Goal:** Terraform configurations for GCP deployment (config only — not deployed).

### Added

- **`infra/terraform/main.tf`** — Terraform provider config (`hashicorp/google ~> 5.0`), `required_version >= 1.5`, commented-out GCS backend template
- **`infra/terraform/variables.tf`** — all config parameterised: `project_id`, `region` (default `europe-west1`), GKE node sizing, autoscaling bounds, BigQuery dataset ID, GCS bucket name, retention days, alert thresholds and notification channel
- **`infra/terraform/outputs.tf`** — exports cluster name, endpoint (sensitive), CA certificate (sensitive), BigQuery table ID, GCS bucket URL, VPC and subnet names
- **`infra/terraform/networking.tf`** — dedicated VPC (`auto_create_subnetworks = false`), GKE subnet with pod (`10.1.0.0/16`) and service (`10.2.0.0/20`) secondary ranges for VPC-native mode, internal firewall, master-to-node firewall, Cloud Router + NAT for egress without public IPs
- **`infra/terraform/gke.tf`** — regional GKE cluster with Workload Identity (`GKE_METADATA` mode), separate node pool (`e2-standard-2`, autoscaling 1–3 nodes, auto-repair + auto-upgrade), VPC-native IP allocation, Cloud Logging + Monitoring integrations, `REGULAR` release channel
- **`infra/terraform/bigquery.tf`** — `pulse_events` dataset, `events` table partitioned by `DATE(timestamp)`, clustered by `customer_id + event_type`, schema mirrors MongoDB collection (8 fields, JSON type for `properties` and `context`)
- **`infra/terraform/gcs.tf`** — exports bucket with versioning enabled, 90-day lifecycle deletion rule, 30-day archived-version deletion rule, uniform bucket-level access
- **`infra/terraform/monitoring.tf`** — three alert policies: API 5xx error rate (>5% over 5 min), consumer Kafka lag (>10k messages), API uptime check (`/health` synthetic monitor); all wired to optional notification channel variable via `locals`
- **`infra/k8s/namespace.yaml`** — `pulse-pipeline` namespace
- **`infra/k8s/api.yaml`** — API `Deployment` (2 replicas), `ConfigMap`, `ServiceAccount` with Workload Identity annotation, `LoadBalancer` Service, `HorizontalPodAutoscaler` (2–5 replicas at 70% CPU), Prometheus scrape annotations, topology spread constraints
- **`infra/k8s/consumer.yaml`** — Consumer `Deployment` (1 replica, 60s termination grace period for graceful flush), `ConfigMap`, `ServiceAccount` with Workload Identity annotation, `ClusterIP` Service for metrics scraping

### Verified

- `terraform validate` passes with no credentials required (`terraform init -backend=false`)

---

## [0.5.0] - 2026-03-24

### Phase 5: Observability 📊

**Goal:** Prometheus metrics, Grafana dashboard, and async API ingestion.

### Added

- **`services/api/kafka/async_producer.go`** — `AsyncProducer` wrapping `kafka.Writer`:
  - Returns immediately from `Produce` / `ProduceBatch` — HTTP handlers never wait on Kafka
  - Background goroutine drains a buffered channel (capacity 10,000) in natural batches
  - Graceful `Close()` drains queue and flushes before shutting down the writer
  - Eliminates EOF errors under high concurrency seen with the sync producer
- **`services/api/metrics/metrics.go`** — `API` struct with 6 metrics on a custom `prometheus.Registry`:
  - `pulse_api_requests_total` (counter_vec: method, path, status)
  - `pulse_api_request_duration_seconds` (histogram_vec: method, path)
  - `pulse_api_events_produced_total` (counter_vec: event_type)
  - `pulse_api_validation_errors_total` (counter)
  - `pulse_api_produce_errors_total` (counter)
  - `pulse_api_queue_depth` (gauge)
- **`services/api/middleware/metrics.go`** — HTTP metrics middleware recording per-request latency and status counts via a `statusRecorder` wrapper
- **`services/consumer/metrics/metrics.go`** — `Consumer` struct with 6 metrics:
  - `pulse_consumer_events_consumed_total` (counter_vec: event_type)
  - `pulse_consumer_dlq_total` (counter)
  - `pulse_consumer_write_duration_seconds` (histogram)
  - `pulse_consumer_writes_total` (counter_vec: status)
  - `pulse_consumer_buffer_size` (gauge)
  - `pulse_consumer_duplicates_skipped_total` (counter)
- **`monitoring/grafana/provisioning/dashboards/pipeline.json`** — 8-panel Grafana dashboard auto-provisioned on startup: events produced/sec, API latency p50/p99, events consumed/sec, MongoDB write latency p50/p99, error rate, events by type, consumer buffer size, DLQ events/sec

### Changed

- **`services/api/main.go`** — uses `AsyncProducer`; exposes `GET /metrics` via `promhttp`; adds metrics middleware; wires `APIMetrics` to handlers
- **`services/api/handlers/track.go`** — accepts `*metrics.API` (nil-safe); increments `events_produced_total` and `validation_errors_total`
- **`services/api/handlers/batch.go`** — accepts `*metrics.API` (nil-safe); increments per-event `events_produced_total` and `validation_errors_total`
- **`services/consumer/kafka/consumer.go`** — accepts `*metrics.Consumer` (nil-safe); tracks `events_consumed_total`, `dlq_total`, `buffer_size`
- **`services/consumer/mongodb/writer.go`** — accepts `*metrics.Consumer` (nil-safe); records `write_duration_seconds` and `duplicates_skipped_total` on each flush
- **`services/consumer/main.go`** — exposes `GET /metrics` on `:8083`; wires `ConsumerMetrics`; default `METRICS_ADDR` fixed to `:8083`
- **`monitoring/prometheus/prometheus.yml`** — fixed consumer scrape target `consumer:8081` → `consumer:8083`
- **`services/api/go.mod` / `services/consumer/go.mod`** — added `github.com/prometheus/client_golang v1.23.2`
- Handler + writer + consumer tests updated to pass `nil` for metrics (nil-safe throughout)

---

## [0.4.0] - 2026-03-24

### Phase 4: Batch Endpoint + Load Testing 📦

**Goal:** Batch event ingestion and basic performance validation.

### Added

- **`services/api/handlers/batch.go`** — `POST /api/v1/track/batch` endpoint:
  - Accepts `{ "events": [...] }` with up to 100 events per request
  - Validates all events first (fail fast with index: `event[N]: <reason>`)
  - Sets defaults (event_id, timestamp, received_at) before producing
  - Returns `202 Accepted` with `{ "accepted": N, "event_ids": [...] }`
  - 1 MB body limit; reuses `KafkaProducer` interface from `TrackHandler`
- **`services/api/handlers/batch_test.go`** — 6 tests: success (3 events), empty array, invalid event at index, exceeds max size (101 events), wrong method, Kafka error, preserved event IDs
- **`scripts/load-test/main.go`** — Go load test: sends 10,000 events via batch endpoint (100 batches × 100 events, concurrency=20); reports throughput (events/sec), p50/p99 batch RTT; exits non-zero if throughput < 500 events/sec
- **`scripts/seed-events/main.go`** — Go seed script: 20 realistic e-commerce events across three customer journeys (alice-001 purchases, bob-002 abandons cart, carol-003 repeat buyer)

### Changed

- **`services/api/main.go`** — registered `POST /api/v1/track/batch` route
- **`Makefile`** — `load-test` and `seed` targets updated to use `go run` (replaced placeholder `.sh` scripts)
- **`scripts/requests.http`** — Phase 4 validation section added (P4-1 through P4-5: batch happy path, error cases, seed, load test)

---

## [0.3.0] - 2026-03-23

### Phase 3: Event Processing 🔄

**Goal:** Consumer reads from Kafka and writes to MongoDB with idempotency.

### Added

- **`services/consumer/models/event.go`** — `Event` and `EventContext` structs with `bson` tags and a `ProcessedAt` timestamp field
- **`services/consumer/mongodb/client.go`** — `Connect()`: establishes MongoDB connection, pings, and creates three indexes on startup:
  - `event_id` (unique) — deduplication guarantee
  - `(customer_id, timestamp desc)` — customer timeline queries
  - `(event_type, timestamp desc)` — event type filtering
- **`services/consumer/mongodb/writer.go`** — `BulkWriter` with idempotent upserts (`$setOnInsert` on `event_id`), unordered bulk writes, and `Add / Flush / Len / Reset` API
- **`services/consumer/kafka/consumer.go`** — `Consumer` batch loop:
  - Accumulates up to 100 messages or flushes every 1 second (whichever comes first)
  - Retries MongoDB flush up to 3 times on error
  - Routes failed batches and unparseable messages to `pulse.events.dlq.v1`
  - Commits Kafka offsets only after successful flush or DLQ routing (at-least-once delivery)
  - Graceful shutdown: flushes remaining buffer before returning
- **`services/consumer/mongodb/writer_test.go`** — 5 tests: buffer management (Add, Len, Reset, empty Flush) + idempotency model test (verifies `$setOnInsert` + `event_id` filter + `Upsert=true`)
- **`services/consumer/kafka/consumer_test.go`** — 8 tests: `parseMessage` (valid, invalid JSON, empty), `flushWithRetry` (success, all-fail, retry-then-succeed), `sendToDLQ` (forwards message, error on write failure)

### Changed

- **`services/consumer/main.go`** — fully wired: MongoDB connection + index setup, `BulkWriter`, `Consumer` loop in goroutine, graceful shutdown sequence (consumer → Kafka → HTTP server → MongoDB disconnect); reads `CONSUMER_BATCH_SIZE` and `CONSUMER_FLUSH_INTERVAL` env vars
- **`services/consumer/kafka/consumer.go`** — `batchSize` and `flushEvery` now constructor parameters (configurable); `dlqSender` interface added for testability; `flushWithRetry()` extracted as package-level function
- **`services/consumer/go.mod`** — added `github.com/segmentio/kafka-go v0.4.50` and `go.mongodb.org/mongo-driver v1.17.9`
- **`docker-compose.yml`** — consumer port changed from `:8081` to `:8083` (`:8081` is occupied by Mongo Express); consumer `ports` mapping added
- **`scripts/requests.http`** — Phase 3 validation section added (P3-1 through P3-6: health check, MongoDB verify, idempotency, burst batching, indexes, consumer group lag)
- **`Makefile`** — `clean` target annotated with warning about volume destruction

---

## [0.2.0] - 2026-03-23

### Phase 2: Event Ingestion 📥

**Goal:** API accepts events, validates them, and produces to Kafka.

### Added

- **`services/api/models/event.go`** — `Event` and `EventContext` structs with full validation:
  - `customer_id`: required, max 256 chars
  - `event_type`: required, must be one of `page_view`, `click`, `purchase`, `add_to_cart`, `search`, `custom`
  - `properties`: optional, max 50 keys, values must be string/number/boolean
  - `SetDefaults()`: generates `evt_<uuid>` if `event_id` missing, fills `timestamp` and `received_at`
- **`services/api/kafka/producer.go`** — Kafka producer wrapper using `segmentio/kafka-go`:
  - Hash partitioning on `customer_id` key for per-customer ordering
  - `RequireAll` acks for durability
  - `MaxAttempts: 3` with graceful `Close()`
- **`services/api/handlers/track.go`** — `POST /api/v1/track` endpoint:
  - 64 KB request body limit
  - Returns `202 Accepted` with `{ "status": "accepted", "event_id": "..." }`
  - Returns `400 Bad Request` with clear validation messages
  - Returns `500 Internal Server Error` on Kafka produce failure
  - `KafkaProducer` interface defined at point of use for testability
- **`services/api/middleware/request_id.go`** — generates 8-byte hex request ID, stores in context, sets `X-Request-ID` response header
- **`services/api/middleware/recovery.go`** — catches panics, logs stack trace, returns `500` JSON response
- **`services/api/models/event_test.go`** — 12 table-driven tests for event validation and `SetDefaults`
- **`services/api/handlers/track_test.go`** — 5 handler tests covering happy path, validation errors, wrong method, Kafka error, and preserved event ID

### Changed

- **`services/api/main.go`** — wired Kafka producer, `TrackHandler`, and middleware chain; graceful shutdown now closes producer
- **`services/api/go.mod`** — added `github.com/segmentio/kafka-go v0.4.50`

---

## [0.1.0] - 2026-03-22

### Phase 1: Foundation 🏗️

**Goal:** Basic project structure, Docker Compose stack, and a working health endpoint.

### Added

- **`docker-compose.yml`** — full local development stack:
  - Kafka 8.2.0 in KRaft mode (no Zookeeper) with `pulse.events.v1` (3 partitions, 7-day retention) and `pulse.events.dlq.v1` (1 partition, 30-day retention) topics created automatically on startup
  - MongoDB 8.2 with persistent volume
  - Prometheus v3.10.0 with scrape config for API (`:8080`) and Consumer (`:8083`)
  - Grafana 12.4.1 with Prometheus datasource auto-provisioned
  - Kafka UI v0.7.2 at `localhost:8082`
  - Mongo Express 1.0.2 at `localhost:8081`
- **`services/api`** — Go 1.26 HTTP service:
  - `GET /health` → `{"status": "ok"}`
  - Structured JSON logging via `log/slog`
  - Graceful shutdown on SIGINT/SIGTERM with 30-second drain timeout
  - Multi-stage Dockerfile (`golang:1.26-alpine` build → `alpine:3.23` runtime, non-root user)
- **`services/consumer`** — Go 1.26 stub service:
  - Health server on `:8081`
  - Graceful shutdown
  - Multi-stage Dockerfile (same pattern as API)
- **`monitoring/prometheus/prometheus.yml`** — scrape config for both services
- **`monitoring/grafana/provisioning/`** — datasource and dashboard provider config (dashboards added in Phase 5)
- **`Makefile`** — targets: `up`, `down`, `logs`, `test`, `lint`, `build`, `load-test`, `seed`, `clean`
- **`.gitignore`** — Go binaries, `.env`, Terraform state, IDE artifacts, macOS files
- **`.env.example`** — all environment variables documented with placeholder values
- **`CHANGELOG.md`** — this file

---

[Unreleased]: https://github.com/zurek11/pulse-pipeline/compare/v0.6.0...HEAD
[0.6.0]: https://github.com/zurek11/pulse-pipeline/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/zurek11/pulse-pipeline/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/zurek11/pulse-pipeline/compare/v0.3.0...v0.4.0
[0.3.0]: https://github.com/zurek11/pulse-pipeline/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/zurek11/pulse-pipeline/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/zurek11/pulse-pipeline/releases/tag/v0.1.0
