# ADR 0002: Modular monolith with three deployables

Status: Accepted
Date: 2026-11-14

## Context
A small team with no live operator access cannot afford the distributed-transaction and operational cost of many services.

## Decision
One Go module with strict package boundaries and three binaries: api, worker, operator-gateway. Extract modules into services only for a named reason.

## Consequences
Simple local development and atomic transactions across modules. Module boundaries must be enforced in CI to keep later extraction cheap.
