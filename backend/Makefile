DATABASE_URL ?= postgres://game:game@localhost:5432/game?sslmode=disable
TEST_DATABASE_URL ?= $(DATABASE_URL)

.PHONY: test test-postgres test-integration sim api worker deps db-up db-down db-bootstrap db-reset smoke

test:
	go test ./...

# Downloads pgx on a machine with internet, then compiles DB/API/worker packages too.
deps:
	go mod download

test-postgres:
	go test -tags postgres ./...

test-integration:
	TEST_DATABASE_URL='$(TEST_DATABASE_URL)' go test -count=1 -p 1 -tags postgres ./...

sim:
	go run ./cmd/simulator

api:
	DATABASE_URL='$(DATABASE_URL)' go run -tags postgres ./cmd/api

worker:
	DATABASE_URL='$(DATABASE_URL)' go run -tags postgres ./cmd/worker

db-up:
	docker compose -f ../docker-compose.yml up -d postgres

db-down:
	docker compose -f ../docker-compose.yml down

db-bootstrap:
	./scripts/bootstrap_db.sh

db-reset:
	./scripts/reset_db.sh

smoke:
	./scripts/smoke_test.sh
