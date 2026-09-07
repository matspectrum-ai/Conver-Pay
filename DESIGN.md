# Conver Pay — Design System

Status: Draft v0.1

## 1. Product character

Conver Pay is a B2B payment orchestration and observability platform. It is not a consumer wallet, bank account, payment gateway, acquirer, or checkout product.

The interface must communicate:

- operational control;
- reliability;
- observability;
- technical precision;
- revenue impact;
- fast incident diagnosis.

The UI must not look like a consumer fintech or a generic SaaS admin template.

## 2. Reference stack

### Primary visual reference — Modern Treasury

Use as the main reference for:

- information density;
- B2B financial operations dashboard;
- navigation hierarchy;
- transaction/payment detail pages;
- data tables;
- filters;
- operational states;
- drill-down workflows;
- high-signal dashboards.

### Functional reference — Primer

Use as the main reference for:

- payment orchestration;
- provider integrations;
- observability;
- payment timelines;
- routing and fallback visualization;
- provider performance comparison;
- transaction lifecycle inspection.

### Developer experience reference — Stripe

Use as a reference for:

- API keys;
- webhook configuration;
- developer settings;
- API documentation;
- request/response inspection;
- test/live environment separation;
- technical copy.

These are references, not clone targets. Conver Pay must have its own visual identity and domain model.

## 3. Visual direction

### Base

- Light-first operational interface.
- Dense but readable information layout.
- Neutral canvas with high-contrast typography.
- Dark sidebar/navigation may be used if it improves hierarchy.
- Minimal decoration.
- No glassmorphism.
- No large gradients as primary UI treatment.
- No oversized marketing-style cards inside the product dashboard.
- No excessive rounded corners.
- No decorative 3D illustrations in operational screens.

### Color semantics

Brand accent should be indigo/cobalt rather than green.

Reason: green is operationally valuable and must remain semantically available for successful payments, healthy providers, and recovered revenue.

Suggested initial semantic roles:

- Brand / primary action: indigo-cobalt.
- Success / healthy / recovered: emerald-green.
- Warning / degraded: amber.
- Failure / unavailable: red.
- Unknown / pending: slate-gray.
- Informational: blue.

Exact color tokens will be finalized after logo/brand exploration.

### Typography

- Sans-serif optimized for dashboards and tabular data.
- Strong numeric legibility.
- Tabular numerals for money, latency, rates, timestamps and IDs.
- Compact type scale.
- Avoid excessively large headings inside authenticated application screens.

## 4. Core product navigation

Initial authenticated navigation:

1. Overview
2. Payments
3. Recoveries
4. Providers
5. Routing
6. Observability
7. Webhooks
8. Developers
9. Organizations
10. Settings

The navigation must reflect jobs-to-be-done, not database tables.

## 5. Overview dashboard

The first screen must answer, within seconds:

- Is payment orchestration healthy right now?
- How much volume is passing through Conver Pay?
- How much revenue did Conver Pay recover?
- Which provider is performing best?
- Is any provider degraded?
- Are payment creation latency or failure rates increasing?

Primary metrics:

- Total payment volume
- Paid volume
- Payment conversion rate
- QR creation success rate
- Median / p95 QR creation latency
- Recovered revenue
- Recovered payments
- Failovers executed
- Provider availability

High-priority widgets:

- Revenue recovered by Conver Pay
- Conversion over time
- Provider performance comparison
- Provider health
- Recent recoveries
- Recent payment failures
- Routing distribution

## 6. Recovered Revenue as a first-class concept

Recovered Revenue is a core product differentiator and must be visible without becoming visually gimmicky.

A payment may be presented as recovered only when there is auditable evidence that:

1. a previous provider attempt failed or met an explicit safe-fallback condition;
2. Conver Pay executed a fallback;
3. a subsequent attempt successfully created a usable payment;
4. that payment was confirmed as paid.

Example UI event:

> R$ 50,00 recovered by Conver Pay
> BlackCat → Provider B
> Trigger: provider unavailable

Recovery cards/events should expose evidence and link to the complete payment timeline.

## 7. Payment list

Use a dense operational table rather than large cards.

Suggested columns:

- Payment ID
- Merchant order ID
- Amount
- Status
- Selected provider
- Attempts
- Recovered
- QR latency
- Created at

Support:

- global search;
- advanced filters;
- date ranges;
- provider filter;
- status filter;
- recovered/not-recovered filter;
- latency filter;
- failure reason filter.

## 8. Payment detail

The payment detail is one of the most important screens.

Structure:

### Summary

- amount;
- current status;
- merchant order ID;
- payment ID;
- selected provider;
- recovered flag;
- creation and payment timestamps.

### Attempt timeline

Visualize each orchestration attempt in order.

Example:

Attempt #1 — BlackCat
- request started;
- timeout threshold reached;
- reconciliation/verification result;
- fallback triggered.

Attempt #2 — Provider B
- request started;
- QR created;
- webhook received;
- payment confirmed.

Every important state change should be timestamped.

### Technical inspection

Where permissions allow:

- normalized request;
- provider request metadata;
- normalized response;
- provider response metadata;
- webhook events;
- routing decision explanation.

Secrets must never be rendered in logs.

## 9. Providers screen

Providers are presented as connections, not as payment accounts owned by Conver Pay.

Each provider row/card should show:

- provider name;
- connection status;
- health status;
- QR creation success rate;
- conversion rate;
- p50/p95 latency;
- recent error rate;
- traffic share;
- environment;
- last successful request.

Connection flow:

1. Select provider.
2. Enter merchant-owned credentials.
3. Validate credentials.
4. Generate/display Conver Pay provider webhook URL.
5. Verify webhook connectivity.
6. Activate connection.

Credentials must be write-only after creation wherever practical.

## 10. Routing screen

Routing must be understandable and auditable.

Initial modes:

### Automatic

Conver Pay chooses the route from health and performance signals.

### Priority

Merchant defines provider order and Conver Pay performs safe failover.

### Rules

Merchant defines explicit routing constraints.

The UI must always expose a human-readable explanation of why a provider was selected.

Example:

> BlackCat selected because it is healthy, has the highest weighted conversion score in the current window, and its p95 QR latency is below the configured SLA.

Avoid opaque AI-style explanations.

## 11. Observability

Observability is not a decorative analytics page. It is an operational surface.

Required views:

- provider availability;
- QR creation latency;
- QR creation success;
- payment conversion;
- error classes;
- timeout rate;
- webhook delivery latency;
- routing distribution;
- fallback rate;
- recovery rate.

Every chart should support drill-down into the payments behind the metric when practical.

## 12. Webhooks

Separate clearly:

### Provider inbound webhooks

Provider → Conver Pay

### Merchant outbound webhooks

Conver Pay → Merchant

Show:

- endpoint;
- signing state;
- delivery status;
- attempts;
- response status;
- latency;
- last error;
- replay action.

## 13. Developer experience

Developer surfaces should be clean and technical.

Include:

- API keys;
- webhook secrets;
- API logs;
- request IDs;
- idempotency keys;
- sandbox/live environment selector;
- API documentation links;
- code examples later.

Never show complete secrets again after creation unless the security model explicitly supports it.

## 14. Interaction principles

- Prefer tables for operational datasets.
- Prefer timelines for payment lifecycle and failover evidence.
- Prefer inline status badges for state.
- Prefer drawers for quick inspection when the user should remain in context.
- Prefer dedicated pages for deep forensic inspection.
- Every destructive action requires explicit confirmation.
- Every routing configuration change should be auditable.
- Never hide important failure information behind generic messages.
- Latency and state transitions should use explicit units and timestamps.

## 15. Anti-patterns

Do not build:

- a Revolut clone;
- a generic shadcn admin dashboard with no product-specific hierarchy;
- a dashboard dominated by vanity metrics;
- arbitrary colorful cards for each KPI;
- fake AI routing visualizations;
- animated payment flows that obscure real operational state;
- provider rankings without sample-size/window context;
- claims of recovered revenue without an auditable recovery event.

## 16. Initial identity hypothesis

Conver Pay should feel like infrastructure that protects revenue.

Working visual direction:

- graphite / off-white neutrals;
- indigo-cobalt brand accent;
- emerald reserved for recovery and healthy/success states;
- precise typography;
- compact cards and tables;
- restrained borders and shadows;
- charts optimized for comparison rather than decoration.

This identity is provisional until the logo and brand tokens are finalized.
