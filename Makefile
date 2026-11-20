# Local settings. Create them once with `make env`; every target below
# then sees them. Do not put comments after a value in .env.local.
-include .env.local
.EXPORT_ALL_VARIABLES:

.PHONY: build test race lint vet vuln tidy up down run-api check-limits check generate migrate env seed reset run-worker

build:
	go build -o bin/ ./cmd/...

test:
	go test ./...

race:
	go test -race -count=1 ./...

check-limits:
	./scripts/check-limits.sh

vet:
	go vet ./...

lint: vet
	golangci-lint run ./...
	staticcheck ./...

generate:
	sqlc generate

vuln:
	@command -v govulncheck >/dev/null && govulncheck ./... || echo "govulncheck not installed, skipped"

tidy:
	go mod tidy

# First run: creates .env.local, starts the containers, migrates and seeds.
# The second make reads .env.local, which did not exist when this one started.
up:
	./scripts/init-env.sh
	docker compose up -d --wait
	$(MAKE) --no-print-directory migrate seed

# Throws the local database away and starts again from scratch.
reset:
	docker compose down -v
	$(MAKE) --no-print-directory up

down:
	docker compose down

env:
	./scripts/init-env.sh

migrate:
	go run ./cmd/migrate up

seed:
	go run ./cmd/zellactl dev seed

run-api:
	go run ./cmd/api

# Sends the emails the API queues, such as merchant invitations.
run-worker:
	go run ./cmd/worker

check: check-limits vet race lint
