# Conver Pay — Architecture

Status: Draft v0.1

## 1. Architectural objective

Conver Pay is a control and orchestration layer between merchant applications and merchant-owned payment providers.

The architecture must optimize for:

- correctness under partial failure;
- deterministic routing;
- safe failover;
- auditable decisions;
- provider abstraction;
- idempotent APIs;
- strong tenant isolation;
- high observability;
- minimal coupling between public API and provider-specific contracts.

The primary failure to prevent is a duplicate payment/charge caused by unsafe retry or fallback.

## 2. High-level architecture

```text
                           ┌────────────────────┐
                           │ Merchant Checkout  │
                           └─────────┬──────────┘
                                     │
                                  HTTPS
                                     │
                                     v
                           ┌────────────────────┐
                           │   Conver Pay API   │
                           └─────────┬──────────┘
                                     │
                         create/load PaymentIntent
                                     │
                                     v
                           ┌────────────────────┐
                           │ Orchestration Core │
                           └──────┬───────┬─────┘
                                  │       │
                           route  │       │ persist state
                                  │       │
                                  v       v
                         ┌────────────┐  ┌──────────────┐
                         │ Router     │  │ Primary DB   │
                         └─────┬──────┘  └──────────────┘
                               │
                       eligible provider
                               │
                               v
                         ┌────────────┐
                         │ Adapter    │
                         │ Interface  │
                         └─────┬──────┘
                               │
                ┌──────────────┼──────────────┐
                v              v              v
          ┌──────────┐   ┌──────────┐   ┌──────────┐
          │Provider A│   │Provider B│   │Provider C│
          └──────────┘   └──────────┘   └──────────┘

Provider webhooks -> Webhook Ingress -> Normalize -> State Machine
                                          |
                                          v
                                  Merchant Webhook Outbox
                                          |
                                          v
                                      Merchant
```

## 3. Control plane vs data plane

### Control plane

Responsible for configuration and administration:

- workspace;
- users/RBAC later;
- provider connections;
- credentials;
- routing configuration;
- API keys;
- webhook endpoints;
- provider enable/disable state;
- dashboard configuration;
- audit log.

### Data plane

Responsible for payment execution:

- public payment API;
- idempotency;
- payment state machine;
- routing;
- provider adapters;
- provider requests;
- provider webhook ingestion;
- reconciliation;
- merchant webhook delivery;
- telemetry;
- recovery attribution.

The payment hot path must not depend on the dashboard or other control-plane UI services.

## 4. Core domain model

### Workspace

Tenant isolation boundary.

Owns provider connections, API keys, payments, webhook endpoints, routing configuration and telemetry.

### ProviderConnection

Represents one merchant-owned account/credential set for one provider and environment.

Suggested fields:

```text
id
workspace_id
provider
status
mode                # live | test
credential_ref      # reference to encrypted secret material
webhook_state
routing_enabled
created_at
updated_at
```

Credentials must not be stored as ordinary plaintext columns.

### PaymentIntent

Canonical Conver Pay identity for one merchant payment request.

Suggested fields:

```text
id
workspace_id
merchant_order_id
idempotency_key
amount
currency
status
active_attempt_id
presented_attempt_id
recovered
recovered_amount
created_at
updated_at
paid_at
expires_at
```

`presented_attempt_id` identifies the attempt whose Pix was actually returned/presented to the customer.

### PaymentAttempt

Represents one interaction with one provider for a PaymentIntent.

Suggested fields:

```text
id
payment_intent_id
provider_connection_id
sequence
status
provider_payment_id
request_started_at
response_received_at
latency_ms
failure_class
fallback_eligibility
reconciliation_status
created_at
updated_at
```

### RoutingDecision

Immutable evidence of why a provider was selected.

```text
id
payment_intent_id
attempt_id
candidate_snapshot
selected_provider_connection_id
score_snapshot
reason_codes
created_at
```

Do not recompute historical explanations from current provider metrics.

### RecoveryEvent

Created only when recovery criteria are satisfied.

```text
id
payment_intent_id
failed_attempt_id
successful_attempt_id
failure_reason
fallback_started_at
successful_qr_at
paid_at
amount
created_at
```

## 5. Payment state machine

Canonical PaymentIntent transitions:

```text
created
   |
   v
routing
   |
   +-----------------------> failed
   |
   v
awaiting_payment
   |
   +-----------> expired
   |
   +-----------> cancelled
   |
   v
paid
```

`paid` is terminal for the payment intent.

The existence of a failed provider attempt does not mean the PaymentIntent is failed if another safe attempt can still produce a customer-facing Pix.

## 6. Attempt state machine

```text
created
   |
   v
requesting
   |
   +---- explicit safe failure ----------> failed_safe
   |
   +---- terminal provider failure ------> failed_terminal
   |
   +---- uncertain network outcome ------> unknown
   |
   +---- usable Pix created -------------> succeeded
```

A successful attempt may later become `superseded` only if business rules explicitly retire it before it is presented. In v1, avoid multiple live usable Pix codes for the same PaymentIntent.

## 7. Why `unknown` is mandatory

Distributed payment systems cannot interpret timeout as absence of side effect.

Example:

```text
Conver Pay -> Provider A: create Pix
Provider A creates Pix
Provider A response is lost
Conver Pay sees timeout
```

Provider A may now contain a valid charge even though Conver Pay did not receive its ID.

Therefore:

- timeout => `unknown`, not `failed_safe`;
- `unknown` blocks normal fallback by default;
- reconciliation or provider-specific idempotency semantics are required before fallback;
- every provider adapter must document which failure classes are safe to retry/fallback.

## 8. Provider adapter contract

Every provider integration implements a common internal contract.

Conceptual interface:

```go
type ProviderAdapter interface {
    CreatePix(ctx context.Context, req CreatePixRequest) (CreatePixResult, error)
    GetPayment(ctx context.Context, ref ProviderPaymentRef) (ProviderPayment, error)
    ReconcileCreate(ctx context.Context, req ReconcileCreateRequest) (ReconcileCreateResult, error)
    ParseWebhook(ctx context.Context, req RawWebhook) (NormalizedWebhookEvent, error)
    VerifyWebhook(ctx context.Context, req RawWebhook) error
    HealthCapabilities() ProviderCapabilities
}
```

The concrete language is not fixed by this document, but the behavioral contract is.

Each adapter must declare capabilities such as:

- provider-side idempotency supported;
- lookup by external merchant reference supported;
- create reconciliation supported;
- webhook signature verification supported;
- cancellation supported;
- expiration supported.

Routing/fallback policy may depend on capabilities.

## 9. Normalized create contract

The orchestration core should not understand provider-specific request bodies.

Internal normalized request:

```text
CreatePixRequest
- payment_intent_id
- merchant_order_id
- amount
- currency
- description?
- customer?
- expires_at?
- provider_idempotency_key
- metadata
```

Internal normalized result:

```text
CreatePixResult
- provider_payment_id
- status
- qr_code_text
- qr_code_image?      # optional; canonical API may prefer text and generate rendering elsewhere
- expires_at
- provider_created_at
- raw_reference
```

Raw provider payloads may be retained for diagnostics subject to retention/security policy, but they must not leak into the public contract.

## 10. Public idempotency

`POST /v1/payment_intents` requires an idempotency key.

Rules:

1. `(workspace_id, idempotency_key)` is unique.
2. Same key + semantically same request returns the original PaymentIntent/result.
3. Same key + conflicting request returns an idempotency conflict.
4. Idempotency is persisted durably, not only cached.
5. Internal provider idempotency keys should derive from immutable attempt identity when supported.

Suggested provider key concept:

```text
cp:{workspace}:{payment_intent}:{attempt}
```

Do not reuse the same provider idempotency key across different provider accounts.

## 11. Routing engine v1

The v1 router is deterministic.

### Eligibility phase

A provider is excluded when any hard rule fails, for example:

- connection disabled;
- wrong environment;
- unhealthy circuit state;
- credentials invalid;
- capability incompatible with request;
- manual exclusion;
- provider currently under enforced cooldown.

### Scoring phase

Eligible providers receive a score from normalized signals.

Illustrative model:

```text
score =
  availability_weight * availability_score
+ qr_success_weight   * qr_success_score
+ conversion_weight   * conversion_score
+ latency_weight      * latency_score
+ preference_weight   * merchant_preference
- error_penalty
```

Actual weights must be config/versioned and evaluated before production use.

### Required safeguards

- minimum sample thresholds;
- metric windows recorded with every decision;
- recent failure penalty;
- circuit breaker;
- deterministic tie-breaker;
- no provider receives conversion advantage from statistically meaningless sample sizes;
- decision snapshot persisted.

## 12. Health model

Do not model health as one boolean.

Provider connection health should expose components:

```text
availability
qr_creation_success
p50_latency
p95_latency
error_rate
timeout_rate
webhook_health
circuit_state
```

Circuit states:

- `closed`: normal routing;
- `open`: provider excluded;
- `half_open`: controlled probes/limited traffic.

The circuit breaker is an operational safety mechanism, not the full routing engine.

## 13. Safe failover algorithm

Conceptual flow:

```text
1. Load/create PaymentIntent idempotently.
2. Acquire orchestration ownership/lock for the intent.
3. Build eligible provider set.
4. Select provider deterministically.
5. Persist RoutingDecision and PaymentAttempt before external call.
6. Call provider through adapter.
7. Classify outcome.
8. If succeeded:
      persist Pix atomically;
      mark attempt succeeded;
      set PaymentIntent awaiting_payment;
      return Pix.
9. If failed_safe:
      persist failure;
      evaluate remaining fallback budget;
      go to step 3 excluding attempted provider.
10. If unknown:
      start reconciliation policy;
      DO NOT normal-fallback until safety is established.
11. If failed_terminal and no safe route remains:
      fail PaymentIntent.
```

## 14. Fallback budget

Every payment execution should have bounded work.

Configuration may include:

- maximum provider attempts;
- overall orchestration deadline;
- per-provider request timeout;
- reconciliation deadline;
- maximum unknown-state wait allowed before returning a non-success response.

Do not hide a long cascade of providers behind an unbounded checkout request.

## 15. Handling slow QR creation

The original product thesis includes switching away from a provider that is too slow.

This must be implemented carefully.

Two different cases exist:

### Pre-request routing

Historical/live p95 latency says Provider A is currently slow.

Safe action: choose Provider B before sending any create request to A.

### In-flight request exceeds latency threshold

Provider A has already received the create request.

Unsafe assumption: cancel locally and immediately create with B.

Correct action depends on provider capabilities:

- if provider supports safe cancellation/lookup/idempotent reconciliation, reconcile first;
- otherwise mark attempt `unknown` and avoid duplicate creation.

Therefore the primary latency optimization should happen before dispatch whenever possible.

## 16. Webhook ingress

Each ProviderConnection receives a distinct Conver Pay inbound webhook route or securely resolvable connection identifier.

Pipeline:

```text
receive
 -> identify provider connection
 -> verify signature/auth
 -> persist raw envelope metadata
 -> parse/normalize
 -> deduplicate event
 -> apply state transition idempotently
 -> enqueue merchant-facing event
 -> acknowledge provider
```

Webhook processing must tolerate duplicate delivery and out-of-order events.

## 17. Merchant webhook delivery

Use an outbox/delivery model.

```text
Domain transaction
  -> persist normalized event + outbox row atomically
  -> async delivery worker
  -> sign payload
  -> POST merchant endpoint
  -> record response/latency
  -> retry using bounded backoff
```

Merchant webhook retries must never replay a domain transition; they replay delivery of an already-persisted event.

Event IDs are stable across retries.

## 18. Webhook event contract

Normalized envelope hypothesis:

```json
{
  "id": "evt_...",
  "type": "payment.paid",
  "created_at": "...",
  "workspace_id": "ws_...",
  "data": {
    "payment_intent_id": "pi_...",
    "merchant_order_id": "order_...",
    "amount": 5000,
    "currency": "BRL",
    "status": "paid",
    "provider": "provider_key"
  }
}
```

A provider-specific raw payload is not the merchant-facing API contract.

## 19. Recovery attribution

Recovery is computed from stored state transitions, never from dashboard analytics alone.

A RecoveryEvent may be created only when:

```text
failed/safe attempt exists
AND successful fallback attempt exists
AND successful fallback attempt is the presented Pix
AND PaymentIntent becomes paid through that presented attempt
```

If attribution is ambiguous, do not mark recovered.

This conservative definition protects trust in the metric.

## 20. Concurrency control

Concurrent create/retry requests for the same PaymentIntent must not produce parallel provider attempts accidentally.

Possible implementation mechanisms:

- database row lock;
- compare-and-swap/version column;
- short-lived distributed lease plus durable state checks.

Prefer a database-backed correctness mechanism first. Distributed locks alone are insufficient as a source of truth.

## 21. Persistence

A relational database is the canonical system of record.

Reasons:

- state-machine consistency;
- transactions;
- unique idempotency constraints;
- auditable relations between payment, attempt and recovery;
- queryability for operations.

PostgreSQL is the default architectural assumption unless a later constraint invalidates it.

A cache such as Redis may be added for:

- short-lived metric materialization;
- rate limiting;
- circuit state acceleration;
- transient leases;
- hot health snapshots.

Redis must not be the only durable record of payment state.

## 22. Metrics pipeline

Separate transactional state from aggregate metrics.

Transactional DB records truth about individual payments.

Metrics pipeline derives windows such as:

- 1 min;
- 5 min;
- 15 min;
- 1 h;
- 24 h;
- configurable longer windows.

Router reads a compact provider score snapshot rather than running expensive analytics on the hot-path database for every checkout request.

Every routing decision records the metric snapshot/version used.

## 23. Secrets and credentials

Provider credentials are high-sensitivity secrets.

Requirements:

- encrypted at rest with envelope/key-management strategy;
- never logged;
- masked after creation;
- access scoped to runtime that needs provider calls;
- rotation supported;
- audit access/change events;
- separation between live and test credentials;
- no secret values in analytics events.

## 24. Auditability

The following changes/events should be auditable:

- provider connected/disconnected;
- credential rotated;
- provider enabled/disabled;
- routing mode changed;
- routing configuration changed;
- circuit manually overridden;
- API key created/revoked;
- webhook endpoint changed;
- payment manually reconciled, if such tooling is later introduced.

RoutingDecision is domain evidence, not merely an admin audit record.

## 25. API/data-plane error classes

Use machine-readable internal/public error classes rather than provider messages as the contract.

Examples:

- `provider_unavailable`
- `provider_timeout_unknown`
- `provider_rejected_safe`
- `provider_invalid_credentials`
- `no_eligible_provider`
- `orchestration_deadline_exceeded`
- `idempotency_conflict`
- `invalid_request`

Raw provider error details belong in protected operational metadata.

## 26. Initial service decomposition

Do not start with microservices.

Recommended MVP deployment shape:

```text
Conver Pay application
├── HTTP API
├── orchestration core
├── routing engine
├── provider adapters
├── webhook ingress
├── background workers
├── admin/dashboard backend
└── telemetry instrumentation

PostgreSQL
Optional Redis
```

This should begin as a modular monolith with explicit package/module boundaries.

Split services only when scaling, reliability or ownership requirements demonstrate a need.

## 27. Verification strategy

Critical behavior must be executable in tests.

Required categories:

### Contract tests

Each provider adapter is tested against a common behavioral contract.

### State-machine tests

Verify legal/illegal PaymentIntent and PaymentAttempt transitions.

### Failover tests

At minimum:

1. safe provider error -> fallback -> Pix success;
2. timeout/unknown -> no unsafe fallback;
3. reconciliation proves absence -> fallback allowed;
4. duplicate client request -> same intent, no second external charge;
5. duplicate webhook -> one domain transition;
6. out-of-order webhook -> state remains valid;
7. paid fallback -> RecoveryEvent created;
8. fallback QR unpaid -> no recovered revenue;
9. provider circuit open -> excluded before dispatch;
10. all providers unavailable -> deterministic terminal error.

### Evals/benchmarks

Before automatic routing is trusted with meaningful traffic, evaluate routing against recorded/synthetic telemetry and compare it with simple baselines such as fixed priority.

## 28. Architectural non-goals

Do not introduce in v1 without evidence:

- LLMs in the payment decision path;
- multi-agent architecture;
- event sourcing for every domain object;
- Kafka solely because this is payments infrastructure;
- microservices per provider;
- active-active multi-region writes before required;
- concurrent race-to-first-QR across providers.

The simplest architecture that preserves payment correctness wins.