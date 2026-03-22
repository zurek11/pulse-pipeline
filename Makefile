.PHONY: up down logs test lint build load-test seed clean

up:
	docker compose up -d

down:
	docker compose down

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
	./scripts/load-test.sh

seed:
	./scripts/seed-events.sh

clean:
	docker compose down -v
	rm -rf bin/
