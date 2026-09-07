# Conver Pay — Product Specification

Status: Draft v0.1

## 1. Product definition

Conver Pay is a payment orchestration platform for Pix.

It connects a merchant to multiple payment gateways/acquirers through one normalized API and dynamically selects which connected provider should receive each payment creation request.

Conver Pay does not custody funds, settle transactions, act as an acquirer, generate Pix independently, or replace the merchant's commercial relationship with each provider. Provider credentials belong to the merchant and are connected to Conver Pay.

The core product promise is:

> Route each payment through the best available provider, fail over safely when a route degrades, and make recovered revenue auditable.

## 2. Core value proposition

A merchant integrating directly with one gateway inherits that gateway's outages, latency spikes and operational failures.

A merchant integrating directly with many gateways inherits integration complexity and still needs to build routing, health monitoring, normalization, reconciliation and failover.

Conver Pay centralizes that layer.

Merchant integration:

```text
Merchant Checkout
      |
      v
   Conver Pay
      |
      +---- Provider A
      +---- Provider B
      +---- Provider C
```

The merchant integrates once with Conver Pay and connects its own provider credentials.

## 3. Product principles

1. Conver Pay is an orchestrator, not a payment processor.
2. The merchant owns provider accounts and credentials.
3. One external API contract should hide provider-specific differences.
4. Routing decisions must be deterministic and auditable in v1.
5. Safe failover is more important than aggressive failover.
6. No payment is labeled recovered without evidence.
7. Provider health is measured from real operational telemetry.
8. Conversion, QR creation success and latency are distinct metrics.
9. Idempotency is mandatory for all payment creation operations.
10. The system must never treat a timeout as proof that a provider did not create a charge.

## 4. Target user

Primary target:

- digital merchants with meaningful Pix volume;
- SaaS/e-commerce/checkout operators integrating Pix through APIs;
- merchants already using or willing to connect more than one provider;
- teams for whom provider instability directly affects conversion.

Secondary target:

- platforms that manage payments for multiple storefronts/products;
- engineering teams that want one normalized Pix integration instead of multiple proprietary integrations.

## 5. Tenant boundary

The canonical isolation boundary is a `Workspace`.

A workspace owns:

- provider connections;
- Conver Pay API keys;
- merchant webhook endpoints;
- routing configuration;
- payments;
- attempts;
- recovery events;
- telemetry;
- audit events.

The data model should allow an account to own multiple workspaces later without requiring a rewrite, but multi-workspace UX is not required for the first implementation milestone.

## 6. MVP capabilities

### 6.1 Provider connections

A merchant can:

- see supported providers;
- connect a provider using merchant-owned credentials;
- validate credentials;
- receive the Conver Pay inbound webhook URL for that connection;
- verify webhook connectivity;
- enable/disable a provider connection;
- see connection and health status.

### 6.2 Unified Pix API

Conver Pay exposes a normalized API to:

- create a Pix payment;
- retrieve a payment;
- retrieve payment attempts;
- receive normalized payment events through merchant webhooks.

The first public API should intentionally be small.

### 6.3 Routing

Initial modes:

- `automatic`: Conver Pay chooses the best eligible route;
- `priority`: merchant supplies an ordered provider preference and Conver Pay performs safe fallback;
- `rules`: reserved for later; the schema may anticipate it, but a rule builder is not required in the first milestone.

### 6.4 Safe failover

Conver Pay may attempt another provider only when the previous attempt reaches a state in which fallback is permitted by policy.

Examples:

- explicit provider rejection before charge creation;
- HTTP/service failure classified as safe-to-retry;
- provider unavailable before a provider charge identifier is observed;
- timeout followed by reconciliation proving no charge exists;
- provider-specific idempotency semantics that make retry/fallback safe.

A network timeout by itself is not a safe-fallback signal.

### 6.5 Webhook normalization

Provider webhook:

```text
Provider -> Conver Pay -> Merchant
```

Conver Pay validates, normalizes and records inbound provider events, then emits a stable merchant-facing event contract.

Example merchant-facing event types:

- `payment.created`
- `payment.pending`
- `payment.paid`
- `payment.failed`
- `payment.expired`
- `payment.recovered`

### 6.6 Observability

The product must measure at least:

- QR creation success rate;
- QR creation latency p50/p95;
- provider request error rate;
- provider timeout rate;
- provider availability;
- payment conversion rate;
- time to payment;
- inbound webhook latency;
- outbound webhook delivery success;
- routing distribution;
- fallback rate;
- recovered payment count;
- recovered revenue.

### 6.7 Recovery evidence

A `RecoveredPayment` is not a marketing inference. It is an auditable event.

Minimum evidence:

1. attempt A failed or entered an explicitly safe fallback condition;
2. Conver Pay initiated fallback;
3. attempt B produced the usable Pix presented to the customer;
4. that Pix was confirmed paid.

The dashboard may then state, for example:

`R$ 50,00 recovered by Conver Pay`

and must link to the underlying attempt timeline.

## 7. Core user journeys

### Journey A — Connect provider

```text
Providers
  -> Select BlackCat
  -> Enter API credentials
  -> Validate
  -> Copy Conver Pay provider webhook URL
  -> Configure provider webhook
  -> Verify
  -> Activate
```

### Journey B — Integrate checkout

```text
Create Conver Pay API key
  -> Call POST /v1/payment_intents
  -> Receive normalized Pix payload
  -> Present QR code / copy-paste code
  -> Receive normalized webhook
```

The merchant should not need provider-specific payment creation code after adopting Conver Pay.

### Journey C — Automatic routing

```text
Payment request
  -> eligible provider set
  -> provider health/performance scoring
  -> route selected
  -> attempt created
  -> provider call
  -> response verified
  -> normalized Pix returned
```

### Journey D — Recovery

```text
Payment request
  -> Provider A selected
  -> safe failure detected
  -> fallback policy permits retry
  -> Provider B selected
  -> Pix created
  -> customer pays
  -> payment.recovered recorded
```

### Journey E — Incident diagnosis

```text
Provider health degrades
  -> routing share changes
  -> merchant opens provider
  -> sees latency/error metrics
  -> drills into affected payments
  -> sees exact attempts and decisions
```

## 8. Public API shape — v1 hypothesis

### Create payment

`POST /v1/payment_intents`

Required concepts:

- merchant order identifier;
- amount in integer centavos;
- currency `BRL`;
- optional customer/reference metadata;
- idempotency key in request header.

Response contains normalized payment identity and Pix data, never raw provider credentials.

### Retrieve payment

`GET /v1/payment_intents/{payment_intent_id}`

### List attempts

`GET /v1/payment_intents/{payment_intent_id}/attempts`

The exact OpenAPI schema is specified separately and is not frozen by this document.

## 9. Payment states

Canonical `PaymentIntent` states:

- `created`
- `routing`
- `awaiting_payment`
- `paid`
- `failed`
- `expired`
- `cancelled`

A payment intent may contain multiple provider attempts but should expose one canonical customer-facing payment state.

## 10. Provider attempt states

Canonical `PaymentAttempt` states:

- `created`
- `requesting`
- `succeeded`
- `failed_safe`
- `failed_terminal`
- `unknown`
- `superseded`

`unknown` is intentionally first-class. It represents cases such as network timeout where provider-side effects are not yet known.

An `unknown` attempt must not automatically cause fallback.

## 11. Routing objective

The router should optimize for successful payment creation and downstream payment conversion subject to operational safety constraints.

Initial score inputs may include:

- provider availability;
- QR creation success rate;
- QR creation latency;
- recent provider error rate;
- payment conversion rate;
- merchant preference;
- minimum sample threshold;
- circuit-breaker state.

No LLM is required or desired in the routing path.

The first version should use explicit weighted scoring plus eligibility rules and circuit breakers.

## 12. Success metrics

Product success should be measured by:

- reduction in failed Pix creation compared with single-provider baseline;
- reduction in p95 QR creation latency during provider degradation;
- recovered revenue with auditable causal evidence;
- provider availability as experienced through Conver Pay;
- merchant integration time;
- webhook delivery reliability;
- duplicate-charge incidents: target zero.

## 13. Non-goals — MVP

Conver Pay does not initially provide:

- custody or wallet balances;
- Pix keys/accounts owned by Conver Pay;
- settlement;
- acquiring;
- card processing;
- product catalog;
- checkout builder;
- subscriptions;
- invoicing;
- marketplace split payments;
- chargeback management;
- AI/LLM-based routing;
- simultaneous race requests to multiple providers;
- automatic provider onboarding/contracting on behalf of merchants.

## 14. Product surfaces

Initial authenticated surfaces:

1. Overview
2. Payments
3. Recoveries
4. Providers
5. Routing
6. Observability
7. Webhooks
8. Developers
9. Settings

All visual implementation must follow `DESIGN.md` as the source of truth.

## 15. Open product questions

These remain intentionally unresolved until implementation requires them:

- pricing model;
- free trial / sandbox policy;
- exact supported provider list for launch;
- minimum data window before conversion affects automatic routing;
- whether merchants may manually override a degraded-provider circuit breaker;
- retention period for raw provider payloads;
- multi-workspace UX and RBAC depth.

These questions should not block the foundation architecture.