# Local settings. Create them once with `make env`; every target below
# then sees them. Do not put comments after a value in .env.local.
-include .env.local
.EXPORT_ALL_VARIABLES:

.PHONY: build test race lint vet vuln tidy up down run-api check-limits check generate migrate env

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

up:
	docker compose up -d --wait

down:
	docker compose down

env:
	./scripts/init-env.sh

migrate:
	go run ./cmd/migrate up

run-api:
	go run ./cmd/api

check: check-limits vet race lint
