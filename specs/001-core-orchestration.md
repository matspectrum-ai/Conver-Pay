# Spec 001 — Core Payment Orchestration

Status: Draft v0.1

## 1. Objective

Implement the smallest end-to-end Conver Pay orchestration slice that can safely accept a Pix payment request, choose one provider, create a normalized payment attempt, return Pix data, process provider webhooks and prove safe fallback behavior.

This spec defines observable behavior. It does not prescribe UI or final framework choices.

## 2. Scope

Included:

- Workspace-scoped API key authentication;
- provider connections;
- normalized `PaymentIntent`;
- normalized `PaymentAttempt`;
- deterministic provider selection;
- one provider adapter contract;
- support for at least two provider adapters in test/fake form so fallback can be exercised;
- public create/retrieve payment API;
- idempotency;
- provider webhook normalization;
- merchant webhook outbox/delivery;
- safe fallback;
- recovery attribution;
- routing decision evidence;
- basic telemetry required by routing.

Excluded:

- dashboard implementation;
- production provider integrations;
- advanced routing rules UI;
- card payments;
- balances/withdrawals;
- settlement;
- LLM/AI routing;
- parallel provider races.

## 3. Invariants

These are hard requirements.

### INV-001 — One idempotent create

For a given `(workspace_id, idempotency_key)`, semantically identical create requests MUST resolve to the same `PaymentIntent`.

They MUST NOT create a second provider-side attempt merely because the client retried the HTTP request.

### INV-002 — Conflicting idempotency key

If an existing idempotency key is reused with a materially different request, the API MUST reject the request with an idempotency conflict.

### INV-003 — Timeout is not safe failure

A provider request timeout MUST NOT be classified as `failed_safe` solely because the local deadline elapsed.

It MUST become `unknown` unless provider-specific evidence establishes a safe state.

### INV-004 — Unknown blocks normal fallback

An `unknown` attempt MUST block ordinary fallback until reconciliation or provider capability proves that creating another Pix cannot produce conflicting live charges.

### INV-005 — One customer-facing Pix

The system MUST NOT intentionally present two simultaneously usable Pix charges for the same `PaymentIntent` in the MVP orchestration path.

### INV-006 — Routing decision evidence

Every provider attempt MUST have an immutable routing-decision record explaining the candidate set, selected provider and score/reason snapshot used at that time.

### INV-007 — Recovery requires paid fallback

A `RecoveryEvent` MUST NOT exist merely because fallback succeeded in creating a QR code.

It requires the fallback-created Pix to become paid.

### INV-008 — Duplicate webhook is harmless

Reprocessing the same provider event MUST NOT duplicate state transitions, recovery events or merchant webhook domain events.

### INV-009 — Public contract is provider-neutral

The public payment response MUST NOT require the merchant to understand the provider-specific API schema.

### INV-010 — Secret isolation

Provider secrets MUST NOT appear in normal logs, public API responses, routing evidence or merchant webhooks.

## 4. Public API — initial behavior

### POST `/v1/payment_intents`

Authentication:

- valid workspace API key required.

Headers:

- `Idempotency-Key` required.

Minimal request hypothesis:

```json
{
  "merchant_order_id": "order_123",
  "amount": 5000,
  "currency": "BRL",
  "metadata": {
    "source": "checkout"
  }
}
```

Minimal successful response hypothesis:

```json
{
  "id": "pi_...",
  "merchant_order_id": "order_123",
  "amount": 5000,
  "currency": "BRL",
  "status": "awaiting_payment",
  "pix": {
    "copy_paste": "000201...",
    "expires_at": "2026-09-07T05:00:00Z"
  },
  "provider": {
    "key": "provider_a"
  },
  "created_at": "2026-09-07T04:00:00Z"
}
```

The exact response schema may evolve before OpenAPI freeze, but provider-native payloads MUST NOT become the public contract.

### GET `/v1/payment_intents/{id}`

Returns canonical payment state.

### GET `/v1/payment_intents/{id}/attempts`

Returns authorized operational attempt summaries. Raw secret material is never returned.

## 5. Routing v1

Routing must happen in two phases.

### 5.1 Eligibility

Exclude provider connections that are:

- disabled;
- wrong environment;
- invalid credentials;
- circuit `open`;
- incompatible with the requested operation;
- already attempted for this intent when policy forbids another attempt;
- explicitly excluded by the current execution.

### 5.2 Ranking

For this milestone, ranking may use a deterministic weighted model.

Minimum signals:

- configured priority;
- provider availability;
- QR creation success;
- p95 creation latency;
- recent error rate.

Conversion can be recorded but SHOULD NOT influence routing until minimum-sample rules are implemented.

Tie-breaking MUST be deterministic.

## 6. Outcome classification

Provider adapters return normalized outcomes.

### Success

Provider confirms usable Pix creation.

Result:

- attempt -> `succeeded`;
- payment -> `awaiting_payment`;
- presented attempt set;
- Pix returned.

### Safe failure

Provider response establishes that no usable charge was created and policy permits another provider attempt.

Result:

- attempt -> `failed_safe`;
- next eligible provider may be selected within fallback budget.

### Terminal failure

Failure cannot be solved by selecting another provider or policy says execution should stop.

Result:

- attempt -> `failed_terminal`;
- payment fails if no other explicit path exists.

### Unknown

Provider-side side effect cannot be determined.

Result:

- attempt -> `unknown`;
- reconciliation starts when supported;
- normal cross-provider fallback blocked.

## 7. Acceptance scenarios

### AC-001 — Normal payment succeeds

Given:

- workspace has Provider A enabled and healthy;
- Provider A is highest-ranked;
- provider create call succeeds.

When:

- merchant calls `POST /v1/payment_intents`.

Then:

- one PaymentIntent is persisted;
- one RoutingDecision is persisted;
- one PaymentAttempt for A is persisted;
- attempt becomes `succeeded`;
- payment becomes `awaiting_payment`;
- normalized Pix is returned;
- no fallback attempt exists.

### AC-002 — Safe failure falls back

Given:

- Provider A ranks above Provider B;
- A returns a classified safe failure;
- B is healthy.

When:

- merchant creates a payment.

Then:

- attempt #1 A -> `failed_safe`;
- routing decision #2 excludes A;
- attempt #2 B is created;
- B succeeds;
- payment becomes `awaiting_payment`;
- B becomes the presented attempt;
- response returns B's normalized Pix.

### AC-003 — Timeout does not immediately fall back

Given:

- Provider A request exceeds local timeout;
- the provider adapter cannot prove whether A created a charge;
- Provider B is healthy.

When:

- merchant creates a payment.

Then:

- attempt A -> `unknown`;
- B MUST NOT be called simply because A timed out;
- payment remains in a non-terminal orchestration/reconciliation state or returns a defined uncertainty error;
- telemetry records timeout/unknown;
- no RecoveryEvent exists.

### AC-004 — Unknown reconciles to not-created, then falls back

Given:

- attempt A is `unknown`;
- adapter reconciliation later proves no provider payment exists;
- Provider B is eligible.

Then:

- A transitions through explicit reconciliation evidence to fallback-eligible state;
- B may be attempted;
- if B succeeds, its Pix is presented;
- audit timeline records reconciliation before fallback.

### AC-005 — Unknown reconciles to created

Given:

- attempt A is `unknown`;
- reconciliation finds that A did create a valid Pix.

Then:

- A becomes the successful/presented attempt;
- B MUST NOT be called;
- normalized Pix for A is returned or made retrievable;
- payment becomes `awaiting_payment`.

### AC-006 — Client retries same create request

Given:

- first request with idempotency key `abc` created `pi_1`;
- provider was already called.

When:

- client retries semantically identical request with `abc`.

Then:

- API resolves to `pi_1`;
- no additional provider call occurs;
- response is consistent with current canonical state.

### AC-007 — Idempotency key conflict

Given:

- key `abc` created amount 5000.

When:

- client reuses `abc` with amount 9000.

Then:

- request is rejected;
- no provider is called;
- original intent is unchanged.

### AC-008 — Payment webhook marks paid

Given:

- presented attempt B exists;
- verified provider webhook reports B paid.

When:

- webhook is processed.

Then:

- provider event is normalized;
- payment becomes `paid` exactly once;
- `paid_at` is recorded;
- merchant-facing `payment.paid` event is placed in outbox.

### AC-009 — Duplicate paid webhook

Given:

- AC-008 already completed.

When:

- same provider webhook/event is delivered again.

Then:

- payment remains paid;
- there is no duplicate domain transition;
- there is no duplicate RecoveryEvent;
- merchant event semantics remain idempotent.

### AC-010 — Recovered payment is attributed

Given:

- A failed safely;
- Conver Pay fell back to B;
- B's Pix was presented;
- B later becomes paid.

Then:

- exactly one RecoveryEvent exists;
- recovered amount equals the PaymentIntent amount;
- timeline links failed A attempt and successful B attempt;
- merchant-facing recovery evidence is queryable;
- `payment.recovered` may be emitted according to event-contract decision.

### AC-011 — Fallback QR unpaid is not recovered revenue

Given:

- A failed safely;
- B produced Pix;
- B remains unpaid/expires.

Then:

- no recovered revenue is counted;
- no RecoveryEvent representing recovered revenue exists.

### AC-012 — Circuit-open provider is skipped before request

Given:

- Provider A is ranked highest historically;
- A circuit is `open`;
- B is eligible.

Then:

- A is removed during eligibility;
- no create call is sent to A;
- B may be selected immediately;
- RoutingDecision records A's exclusion reason.

### AC-013 — No eligible provider

Given:

- all provider connections are disabled/open/incompatible.

Then:

- no external provider call occurs;
- PaymentIntent reaches deterministic failure state;
- API returns a normalized `no_eligible_provider` failure;
- routing evidence explains exclusions.

### AC-014 — Slow historical provider is deprioritized before dispatch

Given:

- A p95 QR latency breaches configured threshold/window;
- B is within threshold;
- no request has yet been sent for this PaymentIntent.

Then:

- router may choose B before dispatch;
- no unsafe in-flight cancellation is required;
- decision snapshot records latency signal.

### AC-015 — In-flight slow request is not treated as proof of failure

Given:

- request has already been sent to A;
- latency SLA passes while response is still unresolved.

Then:

- Conver Pay MUST NOT fire B solely because the SLA was exceeded;
- A outcome must be classified/reconciled under provider capabilities.

## 8. Merchant webhook delivery acceptance

### AC-WH-001 — Stable event ID

Retries of the same merchant webhook delivery MUST preserve the same event ID.

### AC-WH-002 — Signed delivery

Merchant webhook payloads MUST be signed using a workspace/endpoint secret mechanism before production readiness.

### AC-WH-003 — Delivery retry does not replay domain state

A failed merchant endpoint response causes delivery retry only. It MUST NOT re-run provider webhook normalization or payment state transition logic.

### AC-WH-004 — Delivery observability

Each delivery attempt records:

- event ID;
- endpoint ID;
- HTTP response code if available;
- latency;
- attempt number;
- timestamp;
- normalized failure class.

## 9. Provider adapter contract tests

Every real provider adapter MUST pass a shared suite covering:

1. successful create normalization;
2. safe provider error classification;
3. ambiguous timeout classification;
4. webhook verification;
5. webhook normalization;
6. duplicate webhook identity behavior;
7. provider payment lookup when capability is declared;
8. reconciliation behavior when capability is declared;
9. secret redaction in errors/log metadata.

A provider adapter MUST NOT claim a capability that its contract tests do not verify.

## 10. Observability required for milestone completion

At minimum expose internal metrics for:

- create requests total;
- create success/failure/unknown total by provider;
- provider latency histogram;
- fallback count;
- fallback success count;
- unknown outcomes;
- reconciliation outcomes;
- duplicate-idempotency hits;
- provider webhook count/errors;
- merchant webhook delivery count/errors;
- recovered payments total;
- recovered amount total.

Metric labels MUST avoid high-cardinality identifiers such as payment IDs.

## 11. Security acceptance criteria

Before any real provider credentials are used:

- secrets encrypted at rest;
- secrets redacted from logs;
- API keys hashed or otherwise stored with an appropriate one-way/secure representation where applicable;
- workspace access isolation tested;
- webhook verification implemented per provider;
- merchant webhook signing implemented;
- test/live credentials separated;
- sensitive payload retention documented.

## 12. Definition of Done

Spec 001 is complete only when:

- all invariants have automated tests;
- AC-001 through AC-015 pass;
- webhook acceptance criteria pass;
- at least two deterministic fake/provider-simulator adapters exercise fallback paths;
- API contract is represented in OpenAPI;
- state transition tests cover illegal transitions;
- duplicate external-charge regression tests exist;
- CI executes unit, contract and integration suites;
- no production provider credentials are required to run the full test suite;
- a local/demo environment can show a complete normal payment, safe fallback, unknown timeout and recovered-payment flow.

## 13. Explicit production gate

Passing this milestone does NOT authorize production traffic.

Before production:

1. integrate at least one real provider in sandbox/test mode;
2. pass the shared adapter contract suite;
3. validate provider-specific idempotency and reconciliation assumptions against documentation/observed sandbox behavior;
4. load-test orchestration path;
5. verify database/concurrency behavior;
6. verify secret handling;
7. exercise webhook retries and out-of-order events;
8. review automatic-routing weights against a fixed-priority baseline;
9. define operational alerts and rollback/disable controls.
