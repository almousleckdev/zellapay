# ADR 0007: Simulator-first operator development

Status: Accepted
Date: 2026-11-14

## Context
We have no operator access yet and operator behaviour is unreliable and poorly documented.

## Decision
Build a simulator adapter with scripted outcomes (success, failure, slow, duplicate, missing callback, wrong amount) and develop all flows against it. Write real adapters when access is granted.

## Consequences
Development is unblocked. Real operator behaviour will differ; adapter tests must be extended after first contact.
