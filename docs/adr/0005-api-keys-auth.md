# ADR 0005: API keys for merchant authentication

Status: Accepted
Date: 2026-11-14

## Context
Rotating secrets plus short-lived tokens add integration friction and failure modes for merchants.

## Decision
Scoped, hashed API keys with test and live prefixes, optional IP allowlist, and rotation with an overlap window.

## Consequences
Simple integration. Keys must be shown once and protected by merchants.
