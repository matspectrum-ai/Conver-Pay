# Conver Pay

Conver Pay is a payment orchestration layer for merchant-owned gateways and acquirers. It exposes one normalized API, selects eligible providers, performs safe failover, normalizes provider webhooks, and produces auditable recovery evidence.

Conver Pay does not acquire, custody, settle, or independently generate Pix payments.

## Current status

Phase 0 — Foundation.

The repository currently contains the product/architecture specifications and a minimal Go API skeleton with health/readiness endpoints, PostgreSQL connectivity, migrations, structured logging, tests, and CI.

## Stack

- Go API
- PostgreSQL (production target: Supabase Postgres)
- `pgx`
- Goose SQL migrations
- Next.js UI later, after domain/API gates are green
- GitHub Actions CI

No Redis, Kafka, microservices, ML routing, or LLM routing in the MVP.

## Run locally

Requirements: Go 1.27.1+.

```bash
cp .env.example .env
set -a; source .env; set +a
make api-run
```

Without `DATABASE_URL`, `/healthz` is healthy and `/readyz` intentionally returns `503`.

```bash
curl -i http://localhost:8080/healthz
curl -i http://localhost:8080/readyz
```

## Database

Set `DATABASE_URL` to a PostgreSQL connection string and run:

```bash
make db-up
```

Application-owned tables will live under the private `conver_pay` schema.

For Supabase, a persistent Go backend should use a direct database connection when the deployment network supports it; otherwise use Supavisor session mode. Do not use the frontend Data API for payment-domain writes.

## Verification

```bash
make ci
```

See `ROADMAP.md` for gated implementation phases and `specs/001-core-orchestration.md` for the first correctness contract.
