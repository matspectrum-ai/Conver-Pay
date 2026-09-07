# ADR-0001 — Supabase Postgres as initial managed database

Status: Accepted
Date: 2026-09-07

## Context

Conver Pay needs a transactional system of record with explicit constraints, row locking, idempotency records, auditable state transitions, and predictable SQL semantics. The MVP does not need a separate cache, queue, event bus, or database abstraction platform.

## Decision

Use PostgreSQL as the only required datastore. The initial managed production target is Supabase Postgres.

The Go backend connects using a normal PostgreSQL `DATABASE_URL` through `pgx`. Application-owned objects are placed in the private `conver_pay` schema.

Supabase-specific services are not dependencies of the payment core. In particular, the MVP does not require Data API, Edge Functions, Realtime, Storage, Queues, or direct browser access to payment-domain tables.

For a persistent Go backend:

- prefer Supabase direct connection when the deployment network can reach it;
- otherwise use Supavisor session mode on port 5432;
- do not use transaction pooling unless the deployment model becomes transient/serverless and its limitations are explicitly accepted.

## Consequences

Positive:

- full PostgreSQL semantics;
- low operational burden;
- easy local development and portability;
- no vendor-specific database API in domain code;
- fewer moving parts.

Trade-offs:

- PostgreSQL initially carries durable job/outbox responsibilities too;
- high-frequency ephemeral telemetry may later justify Redis or another specialized store;
- moving away from Supabase still requires operational migration, even though application code remains PostgreSQL-oriented.

## Exit criteria for another datastore

Add another datastore only when a measured requirement cannot be met cleanly with PostgreSQL, and document the decision in a new ADR.
