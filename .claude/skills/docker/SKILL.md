---
name: docker
description: 'REQUIRED when modifying Dockerfile, docker-compose.yml, or container configuration.'
allowed-tools: Read, Write, Edit, Bash(docker:*), Bash(go:*)
---

# Docker Patterns

## Dockerfile — Multi-Stage Build (Go)

```dockerfile
# Stage 1: Build
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /service .

# Stage 2: Production
FROM alpine:3.19
RUN adduser -D -g '' appuser
COPY --from=build /service /service
USER appuser
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=3s CMD wget -qO- http://localhost:8080/health || exit 1
ENTRYPOINT ["/service"]
```

## Docker Compose Services

```yaml
# Required services for docker-compose.yml:
# - zookeeper (Kafka dependency)
# - kafka (event streaming)
# - kafka-init (create topics on startup)
# - kafka-ui (topic browser — port 8082)
# - mongodb (event storage — port 27017)
# - mongo-express (data browser — port 8081)
# - prometheus (metrics — port 9090)
# - grafana (dashboards — port 3000)
# - api (Go API — port 8080)
# - consumer (Go consumer — internal only)
```

## Rules

1. Multi-stage builds — keep production image under 20MB
2. `CGO_ENABLED=0` — static binary, no libc dependency
3. Run as non-root user (`appuser`)
4. Health check in every service Dockerfile
5. Use `alpine` for runtime (not `golang` image)
6. Pin versions: `golang:1.22-alpine`, `alpine:3.19`
7. `.dockerignore` must exclude: `.git`, `*.md`, `.claude`, `infra/`, `docs/`
8. Environment variables for all configuration (no hardcoded values)
9. Docker Compose: use `depends_on` with health checks for startup ordering
10. Kafka UI at `:8082`, Mongo Express at `:8081` — for development only
