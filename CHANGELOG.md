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
  - Prometheus v3.10.0 with scrape config for API (`:8080`) and Consumer (`:8081`)
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

[Unreleased]: https://github.com/zurek11/pulse-pipeline/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/zurek11/pulse-pipeline/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/zurek11/pulse-pipeline/releases/tag/v0.1.0
