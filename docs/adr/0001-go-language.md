# ADR 0001: Go as the implementation language

Status: Accepted
Date: 2026-11-14

## Context
We need predictable latency, many concurrent operator calls, simple deployment and strong library support for Postgres, gRPC and OpenTelemetry. Alternatives considered: Java/Kotlin (heavier to run, slower to iterate), Rust (slower to hire for and build), Node/TypeScript (kept for frontends only).

## Decision
Use Go for all backend services.

## Consequences
Single static binaries and a small operational footprint. Fewer hiring options than Java in some markets. Frontends remain TypeScript.
