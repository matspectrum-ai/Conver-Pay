# AGENTS.md — Conver Pay

## Source of truth

Read before changing behavior:

1. `PRODUCT.md`
2. `ARCHITECTURE.md`
3. `specs/001-core-orchestration.md`
4. `ROADMAP.md`
5. `DESIGN.md` only for visual work

`DESIGN.md` is an imported Raycast design analysis and must not be edited to encode Conver Pay domain behavior.

## Engineering model

Use: Observe -> Plan -> Act -> Verify -> Iterate.

Prefer the smallest architecture that satisfies the specs. The current target is a modular monolith with a Go API and PostgreSQL.

Do not introduce Redis, Kafka, microservices, Temporal, Kubernetes, ML routing, LLM routing, or concurrent race-to-first-QR without a measured requirement and an ADR.

## Payment correctness invariants

- A timeout is not proof that a provider did not create a payment.
- Never perform cross-provider fallback from an ambiguous/unknown attempt until reconciliation or a provider capability proves it safe.
- Every merchant-facing create operation must be idempotent.
- `PaymentIntent` and `PaymentAttempt` are distinct concepts.
- Historical routing decisions are immutable evidence; do not recompute them from current telemetry.
- `recovered` revenue requires a failed/safe prior attempt, Conver Pay fallback, a successful later attempt, and confirmed payment.
- Duplicate and out-of-order webhooks must not corrupt state.
- Secrets must never be logged.

## Database

- PostgreSQL is the system of record.
- Production hosting target: Supabase Postgres, accessed through `DATABASE_URL` from the Go backend.
- Application data belongs in the private `conver_pay` schema, not `public`.
- Do not make the frontend depend on Supabase Data API for payment-domain state.
- Prefer explicit SQL, transactions, constraints, and row locking where correctness requires them.
- `pgx` is the runtime driver. `sqlc` may be introduced with Phase 1 queries.

## Provider adapters

Provider-specific payloads stop at the adapter boundary. Core orchestration code works with normalized domain types.

Each real adapter must document and test:

- create-payment semantics;
- provider idempotency support;
- timeout ambiguity behavior;
- reconciliation capability;
- webhook authentication;
- duplicate webhook behavior;
- provider-specific error classification.

## Commands

From repository root:

- `make api-run`
- `make api-test`
- `make api-vet`
- `make api-fmt`
- `make ci`
- `make db-up` (requires `DATABASE_URL`)

## Change discipline

- Specs before complex behavior.
- Tests before or with implementation for state transitions and failure handling.
- Do not weaken an invariant just to make a test pass.
- A feature is incomplete until its failure path is testable.
- Keep external dependencies minimal and pinned.
