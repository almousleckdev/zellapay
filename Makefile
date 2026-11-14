.PHONY: build test race lint vet vuln tidy up down run-api check-limits check generate

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

run-api:
	go run ./cmd/api

check: check-limits vet race lint
