# Conver Pay — Engineering Roadmap

Status: Draft v0.1

This roadmap is gated. A later phase should not compensate for an unverified earlier phase.

## Phase 0 — Foundation

Goal: establish repository, engineering contracts and CI before product code grows.

Deliverables:

- repository conventions;
- `AGENTS.md`;
- local development bootstrap;
- lint/format/test commands;
- CI pipeline;
- environment configuration contract;
- architecture package boundaries;
- database migration mechanism;
- structured logging conventions;
- error taxonomy skeleton.

Gate:

- clean checkout can bootstrap and run tests;
- CI green;
- no production credentials required.

## Phase 1 — Domain Core

Goal: implement payment correctness without any real gateway.

Deliverables:

- Workspace;
- ProviderConnection model;
- PaymentIntent state machine;
- PaymentAttempt state machine;
- RoutingDecision;
- RecoveryEvent;
- persistent idempotency;
- deterministic fake provider adapters;
- orchestration service;
- safe fallback classification;
- unknown/reconciliation flow.

Gate:

- invariants in `specs/001-core-orchestration.md` pass;
- duplicate-charge regression suite green.

## Phase 2 — Public API + Webhooks

Goal: expose a stable merchant-facing vertical slice.

Deliverables:

- API authentication;
- `POST /v1/payment_intents`;
- retrieve payment;
- retrieve attempts;
- OpenAPI;
- provider webhook ingress abstraction;
- merchant webhook outbox;
- delivery retries;
- signing;
- delivery logs.

Gate:

- full fake-provider checkout flow works through HTTP;
- duplicate and out-of-order webhook tests pass;
- OpenAPI contract tests green.

## Phase 3 — Routing Telemetry

Goal: make automatic routing data-driven without ML/LLM opacity.

Deliverables:

- provider health snapshots;
- latency histograms;
- QR creation success;
- error/timeout rates;
- circuit breaker;
- eligibility filtering;
- deterministic weighted scoring;
- score/version snapshot persisted per routing decision.

Gate:

- automatic router evaluated against fixed-priority baseline using deterministic simulations;
- circuit behavior tests green;
- no high-cardinality metric design errors.

## Phase 4 — First Real Provider

Goal: validate abstractions against reality.

Deliverables:

- first production-grade provider adapter;
- sandbox/test integration;
- credential validation;
- webhook verification;
- provider-specific failure classifier;
- provider-specific idempotency/reconciliation capability matrix;
- adapter contract tests.

Gate:

- documented sandbox evidence for create/idempotency/webhook/reconciliation behavior;
- no assumptions based only on API documentation when observable sandbox behavior can validate them.

## Phase 5 — Second Real Provider + Real Failover

Goal: prove cross-provider orchestration.

Deliverables:

- second production-grade adapter;
- cross-provider safe fallback tests;
- provider degradation simulator;
- recovery attribution end-to-end;
- fallback budget tuning.

Gate:

- safe A -> B fallback demonstrated;
- ambiguous A timeout does not produce unsafe B call;
- recovered payment evidence complete.

## Phase 6 — Product UI

Goal: expose the already-verified operational model through the Conver Pay interface.

Visual source of truth: `DESIGN.md`.

Initial surfaces:

- Overview;
- Payments;
- Payment detail/timeline;
- Recoveries;
- Providers;
- Routing;
- Observability;
- Webhooks;
- Developers;
- Settings.

Gate:

- UI reads real API/domain data rather than mock-only structures;
- recovery claims always link to evidence;
- routing explanations reflect persisted RoutingDecision snapshots.

## Phase 7 — Production Readiness

Goal: qualify the system for real merchant traffic.

Deliverables:

- load tests;
- concurrency stress tests;
- secret-management review;
- backup/restore drill;
- operational alerts;
- SLOs;
- rate limiting;
- audit logging;
- credential rotation;
- provider kill switch;
- routing rollback controls;
- incident runbooks;
- webhook replay tooling;
- data retention policy.

Gate:

- production-readiness review passes;
- known failure modes have tested operational responses;
- production deployment can be rolled back/disabled safely.

## Deferred until evidence justifies them

- microservice decomposition;
- Kafka/event streaming infrastructure;
- multi-region active-active writes;
- ML routing;
- LLM routing;
- multi-agent systems;
- concurrent race-to-first-QR;
- advanced rules builder;
- cards and other payment methods;
- sophisticated organization/RBAC hierarchy.
