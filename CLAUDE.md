# Pulse Pipeline

## About

Learning project for Go, Apache Kafka, MongoDB, and GCP — a mini real-time event tracking pipeline for exploring distributed systems concepts.

**⚠️ This is a PUBLIC repository.** Never commit secrets, API keys, .env files, or any sensitive data.

- **Author:** Adam Žúrek (https://github.com/zurek11)
- **Repo:** https://github.com/zurek11/pulse-pipeline
- **Spec:** See `docs/PROJECT_SPEC.md` for full project specification, architecture, phases, and acceptance criteria. Read this file BEFORE starting any work.

## Stack

- **Language:** Go 1.26+ (standard library preferred, minimal dependencies)
- **Streaming:** Apache Kafka (Confluent Go client)
- **Storage:** MongoDB (official Go driver)
- **Metrics:** Prometheus (client_golang)
- **Dashboards:** Grafana (dashboards as JSON)
- **IaC:** Terraform (GCP provider — config only, not deployed)
- **Containers:** Docker + Docker Compose
- **CI:** GitHub Actions

## Project Structure

```
pulse-pipeline/
├── services/
│   ├── api/                   # HTTP Tracking API
│   │   ├── main.go            # Entry point, server setup, graceful shutdown
│   │   ├── go.mod / go.sum
│   │   ├── Dockerfile
│   │   ├── handlers/          # HTTP route handlers
│   │   ├── middleware/        # Logging, metrics, panic recovery
│   │   ├── models/            # Event structs, validation
│   │   └── kafka/             # Kafka producer wrapper
│   └── consumer/              # Kafka Consumer → MongoDB writer
│       ├── main.go            # Entry point, consumer loop, graceful shutdown
│       ├── go.mod / go.sum
│       ├── Dockerfile
│       ├── kafka/             # Kafka consumer wrapper
│       ├── mongodb/           # MongoDB bulk writer
│       └── models/            # Shared event models
├── infra/
│   ├── terraform/             # GCP IaC (not deployed, config only)
│   └── k8s/                   # Kubernetes manifests
├── monitoring/
│   ├── prometheus/            # Scrape config
│   └── grafana/               # Dashboard provisioning + JSON
├── scripts/                   # Load test, seed data
└── docker-compose.yml         # Full local stack
```

## Commands

### Docker Compose (full stack)

- `docker compose up -d` — start everything (Kafka, MongoDB, Prometheus, Grafana, API, Consumer)
- `docker compose down` — stop everything
- `docker compose logs -f api consumer` — tail service logs
- `docker compose down -v` — stop + remove volumes (clean slate)

### Go development

- `cd services/api && go run .` — run API server locally
- `cd services/consumer && go run .` — run consumer locally
- `cd services/api && go test ./...` — run API tests
- `cd services/consumer && go test ./...` — run consumer tests
- `golangci-lint run ./...` — lint (from service directory)

### Make shortcuts

- `make up` / `make down` — start/stop stack
- `make test` — test all services
- `make lint` — lint all services
- `make build` — build Go binaries
- `make load-test` — send 1000 events

### Terraform (config only — NOT deployed)

- `cd infra/terraform && terraform init` — initialize
- `cd infra/terraform && terraform plan` — preview (requires GCP credentials)
- `cd infra/terraform && terraform validate` — validate syntax (no credentials needed)

## Code Conventions

### Go

- Standard library first — avoid dependencies where stdlib suffices
- `net/http` for HTTP server (no frameworks like Gin/Echo)
- Explicit error handling — always check and wrap errors with `fmt.Errorf("context: %w", err)`
- Structured logging via `log/slog` (stdlib, not logrus/zap)
- Context propagation — pass `context.Context` as first parameter everywhere
- Graceful shutdown — listen for SIGINT/SIGTERM, drain connections, close Kafka/MongoDB
- Table-driven tests — use `[]struct{ name, input, want }` pattern
- No `panic()` in production code — return errors
- No global state — pass dependencies via constructor injection
- Package names: short, lowercase, no underscores (e.g., `kafka`, `mongodb`, `handlers`)
- File names: snake_case (e.g., `event_handler.go`, `bulk_writer.go`)
- Interfaces: define at point of use, not at implementation

### Docker

- Multi-stage builds — separate build and runtime stages
- Use `golang:1.22-alpine` for build, `alpine:3.19` for runtime
- Copy only the binary to runtime stage
- Run as non-root user
- Health check in Dockerfile

### Terraform

- One resource type per file (gke.tf, bigquery.tf, gcs.tf)
- Variables in `variables.tf` with descriptions and defaults
- Outputs in `outputs.tf`
- Use `google` provider, region `europe-west1` (closest to Bratislava)
- Resource names prefixed with `pulse-pipeline-`
- No hardcoded values — everything parameterized

### Kafka

- Topic naming: `pulse.events.v1` (namespace.entity.version)
- Key: `customer_id` (ensures ordering per customer)
- Value: JSON-encoded event
- Producer: sync writes with acks=all for durability
- Consumer: at-least-once delivery + idempotent MongoDB writes

### MongoDB

- Database: `pulse`
- Collection: `events`
- Indexes: `{ customer_id: 1, timestamp: -1 }`, `{ event_type: 1 }`, `{ _id: 1 }` (natural)
- Bulk writes with `ordered: false` for throughput
- Idempotent upserts using event_id as deduplication key

## Git Workflow

### Commit Messages — Emoji Conventional Commits

```
🎉 feat: add event tracking HTTP endpoint
🐛 fix: resolve Kafka producer timeout on high load
♻️ refactor: extract MongoDB bulk writer into package
🧪 test: add table-driven tests for event validation
📝 docs: add architecture diagram to README
🔧 chore: update Go dependencies
🐳 docker: optimize multi-stage build for API
🏗️ infra: add GKE Terraform configuration
📊 metrics: add events_processed_total counter
🚀 release: v0.3.0 — full pipeline with observability
```

### Branch Flow

- `main` — stable, versioned releases only
- `feat/[description]` — new features
- `fix/[description]` — bug fixes
- `infra/[description]` — infrastructure changes
- Feature branches → PR → main. Adam reviews and merges.

### PR Workflow

1. Create branch from main
2. Implement + write tests
3. Run: `make test && make lint`
4. Update CHANGELOG.md (move items from `[Unreleased]` to new version section)
5. Commit + push
6. Create PR — see **git-workflow** skill for exact title/body format
   - Release PRs: title `🚀 Release v{version} — {phase}`, body = CHANGELOG section
   - Feature PRs: title with emoji prefix, body with summary + test plan

## Claude Code Configuration

### Skills (`.claude/skills/`)

Skills are NOT auto-applied. Claude Code MUST invoke each skill explicitly before the corresponding work begins.

| Skill | MUST invoke before… |
|---|---|
| **go-service** | creating any new `.go` file or package |
| **kafka-patterns** | writing Kafka producer or consumer code |
| **mongodb-patterns** | writing MongoDB read/write operations |
| **docker** | editing Dockerfile or docker-compose.yml |
| **terraform** | editing any `.tf` file |
| **testing** | creating or modifying `_test.go` files |
| **git-workflow** | any `git push` or `gh pr create` |
| **observability** | adding metrics, dashboards, or alerting |

### Rules (`.claude/rules/`)

- **security.md** — public repo safety rules (always active)

## Important Notes

- All infrastructure runs locally via Docker Compose — no cloud costs
- Terraform configs are "ready to deploy" but NOT actually deployed
- Kafka UI available at `http://localhost:8082` for topic inspection
- MongoDB Express at `http://localhost:8081` for data browsing
- Grafana at `http://localhost:3000` (admin/admin) for dashboards
- Prometheus at `http://localhost:9090` for raw metrics
- API server at `http://localhost:8080`
