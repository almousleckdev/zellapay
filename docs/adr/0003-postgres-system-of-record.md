# ADR 0003: PostgreSQL as system of record, Postgres-backed jobs

Status: Accepted
Date: 2026-11-14

## Context
The ledger needs ACID transactions and row-level locking. Job enqueue should share a transaction with business writes.

## Decision
Use PostgreSQL 17 with sqlc and pgx, goose migrations, and river for background jobs. Redis is used only for rate limits, idempotency fast path and short locks.

## Consequences
One fewer system to run in phase 1. Throughput of the job queue is bounded by Postgres; revisit with NATS JetStream if needed.
