.PHONY: up down rebuild logs test lint build load-test seed clean

up:
	docker compose up -d

down:
	docker compose down

rebuild:
	docker compose up -d --build api consumer

logs:
	docker compose logs -f api consumer

test:
	cd services/api && go test ./...
	cd services/consumer && go test ./...

lint:
	cd services/api && golangci-lint run ./...
	cd services/consumer && golangci-lint run ./...

build:
	mkdir -p bin
	cd services/api && go build -o ../../bin/api .
	cd services/consumer && go build -o ../../bin/consumer .

load-test:
	go run scripts/load-test/main.go

seed:
	go run scripts/seed-events/main.go

clean: ## WARNING: destroys all volumes (MongoDB data, Kafka, Grafana, Prometheus)
	docker compose down -v
	rm -rf bin/
