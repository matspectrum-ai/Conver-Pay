# Conver Pay — Design System

Status: Draft v0.2

## 1. Product character

Conver Pay is a payment orchestration platform. It is not a consumer wallet, bank, payment gateway, acquirer, or checkout product.

The product should feel like a new control layer for payments: fast, intelligent, technical, alive, and trustworthy.

The interface must communicate:

- orchestration in real time;
- revenue protection;
- speed;
- observability;
- technical precision;
- confidence under failure;
- operational control.

The UI must not look like a legacy financial backoffice, a generic admin template, or a consumer banking clone.

## 2. Reference stack

Conver Pay does not have a single clone target. Its visual language is composed from modern product references while its domain behavior comes from payment infrastructure products.

### Primary visual language — Linear

Use as the main reference for:

- calm high-density interfaces;
- strong hierarchy with low visual noise;
- compact navigation;
- dark surfaces;
- command-driven interaction;
- keyboard-first behavior;
- predictable headers and view controls;
- subtle motion and transitions;
- crisp icons and typography.

### Surface and motion reference — Raycast

Use as a reference for:

- premium dark UI;
- layered depth;
- tasteful translucent/elevated surfaces;
- command palette behavior;
- responsive micro-interactions;
- interface scale and rhythm;
- motion that reinforces state rather than decorating it.

### Data workspace reference — Attio

Use as a reference for:

- flexible split views;
- resizable panels;
- dense record inspection;
- contextual sidebars;
- activity timelines;
- powerful filters;
- inline editing without losing context.

### Payment orchestration reference — Primer

Use as the domain reference for:

- provider integrations;
- routing and fallback concepts;
- payment attempts;
- observability;
- provider performance;
- payment lifecycle inspection.

### Developer experience reference — Stripe / Vercel

Use as references for:

- API keys;
- webhook configuration;
- API logs;
- environments;
- request inspection;
- deployment-like operational confidence;
- developer-focused copy and information hierarchy.

These products are references, not clone targets. Conver Pay must be immediately recognizable as Conver Pay.

## 3. Visual thesis — Payment Operations Cockpit

The authenticated product should feel less like a dashboard and more like an operational cockpit.

The user should perceive that money is moving through a live network and that Conver Pay is continuously making routing decisions to protect conversion.

The visual system should combine:

- near-black / graphite foundation;
- high-contrast content surfaces;
- one distinctive electric brand accent;
- semantic colors used sparingly;
- thin borders and subtle elevation;
- compact information density;
- real-time state changes;
- high-quality charts and topology views;
- minimal but deliberate depth/translucency.

Avoid visual noise. Modern does not mean decorative.

## 4. Theme

### Default authenticated theme

Dark-first.

Rationale: the product is operational infrastructure and should read as a high-performance control surface rather than an accounting portal.

A light theme may exist, but the canonical product identity is dark.

### Surface hierarchy

- Canvas: near-black.
- Navigation: slightly differentiated graphite.
- Primary surface: elevated dark neutral.
- Secondary surface: subtle translucent or tonal layer.
- Hover/focus surface: controlled luminance increase.
- Borders: low-contrast by default, stronger on focus/selection.

Translucency may be used selectively for command surfaces, floating inspectors, overlays, and real-time control layers. Do not apply glass effects indiscriminately to every card.

## 5. Brand color hypothesis

The primary brand accent should not be generic fintech blue and should not consume green, which is needed for payment success and recovery semantics.

Working direction:

- Brand: electric violet / ultraviolet.
- Secondary signal: cold cyan.
- Success / recovered / healthy: emerald.
- Warning / degraded: amber.
- Failure / unavailable: red.
- Pending / unknown: slate.

The exact tokens will be fixed during brand exploration.

The accent may appear as a restrained glow only in high-value contexts such as the active routing path, selected provider, command focus, or key recovery moment.

No rainbow gradients. No neon overload.

## 6. Typography

The typography must feel engineered, not editorial.

Requirements:

- compact sans-serif optimized for interfaces;
- excellent numeric legibility;
- tabular numerals for currency, latency, rates and timestamps;
- monospace for IDs, API keys, request IDs, webhook signatures and technical payloads;
- compact type scale inside the app;
- strong hierarchy created primarily by weight, contrast and spacing rather than oversized headings.

## 7. Product shell

The application shell should be persistent and highly responsive.

Recommended structure:

- compact left navigation;
- global command/search trigger;
- workspace / organization switcher;
- environment selector (`LIVE` / `TEST`);
- real-time system health indicator;
- page-level command bar;
- main content area with optional inspector/sidebar.

The shell should support collapsed navigation and keyboard-driven navigation.

## 8. Command palette

A command palette is a first-class interaction, not a novelty.

Example actions:

- Go to payment `pi_...`
- Search merchant order
- Open provider BlackCat
- Disable provider
- Change routing mode
- Replay webhook
- Copy API request
- Open recovery
- Switch organization
- Switch environment

Power users should be able to move through the product without navigating several menus.

## 9. Overview — live orchestration surface

The Overview should not be a wall of KPI cards.

It should answer immediately:

- Is routing healthy now?
- What is happening to conversion now?
- Which provider is winning traffic?
- Is a provider degrading?
- How much revenue has Conver Pay recovered?
- Are failovers happening abnormally often?

Recommended composition:

### Top signal bar

A narrow live status layer:

`System Healthy · 4 providers online · p95 QR 284 ms · 2 active incidents`

### Core performance

Use a restrained metric strip for:

- Payment volume
- Conversion
- QR success
- Recovered revenue
- Failovers

### Routing topology

A distinctive live visualization showing traffic flowing from Conver Pay to active providers.

The topology must show real data, not decorative animation.

Example concepts:

- traffic share;
- current route priority;
- provider health;
- live latency;
- active degradation;
- failover paths.

### Recovery feed

A live feed of verified recoveries:

`R$ 197 recovered · BlackCat → Provider B · timeout · 18s ago`

Each event opens the complete evidence timeline.

## 10. Routing topology as a signature UI

Routing is the product's core capability and should have a visual representation unique to Conver Pay.

The topology may visually express:

```text
Checkout traffic
      ↓
  Conver Pay
   ╱   │   ╲
  ╱    │    ╲
 A     B     C
72%   18%   10%
```

But the production UI should use modern nodes, live health rings, traffic paths, latency labels and failover events.

Rules:

- no fake packet animations;
- no meaningless particle effects;
- route animation only when it communicates a real state transition;
- all visual states must map to actual telemetry;
- clicking a node reveals the provider inspector;
- clicking a path reveals the routing decision and sample payments.

This topology is intended to become one of Conver Pay's recognizable product signatures.

## 11. Recovered Revenue as a first-class concept

Recovered Revenue is a core product differentiator.

A payment may only be labeled recovered when there is auditable evidence that:

1. a previous provider attempt failed or met an explicit safe-fallback condition;
2. Conver Pay executed a fallback;
3. a subsequent attempt successfully produced a usable Pix payment;
4. that payment was confirmed as paid.

Recovery UI should feel consequential but not gamified.

Example:

`R$ 50,00 recovered by Conver Pay`

Supporting evidence:

`BlackCat → Provider B`
`Trigger: provider unavailable`
`Fallback latency: 214 ms`

## 12. Payments view

Use an ultra-responsive operational table/list.

Suggested columns:

- Payment ID
- Merchant order ID
- Amount
- Status
- Route
- Attempts
- Recovered
- QR latency
- Created at

Required behavior:

- instant search;
- keyboard selection;
- advanced filters;
- saved views;
- column configuration;
- density toggle;
- sticky headers;
- multi-select actions where safe;
- quick inspector without leaving the list.

Opening a payment should be possible either as a side inspector or a full forensic page.

## 13. Payment forensic view

The payment detail is one of the flagship screens.

Use a split-view inspired workflow:

### Main timeline

Chronological orchestration lifecycle.

Example:

`01:42:12.108  ROUTE_SELECTED     BlackCat`
`01:42:12.903  SLA_EXCEEDED       795 ms`
`01:42:12.918  FALLBACK_STARTED   Provider B`
`01:42:13.191  QR_CREATED         273 ms`
`01:44:51.227  WEBHOOK_RECEIVED`
`01:44:51.231  PAYMENT_PAID`

### Inspector panel

Contextual details:

- amount;
- merchant order;
- selected provider;
- score inputs;
- attempts;
- normalized request;
- provider response metadata;
- webhook events;
- idempotency key;
- request IDs.

Panels should be resizable and collapsible.

## 14. Providers

Providers should feel like infrastructure nodes, not marketplace cards.

Each connection shows:

- current status;
- health;
- traffic share;
- QR success;
- conversion;
- p50/p95 latency;
- error rate;
- last successful request;
- environment.

A provider detail view should combine live metrics, incidents, recent requests and configuration.

Connection flow:

1. Choose provider.
2. Add merchant-owned credentials.
3. Validate connection.
4. Receive Conver Pay inbound webhook URL.
5. Verify webhook connectivity.
6. Activate provider.

Credentials are write-only after creation wherever practical.

## 15. Routing

Initial routing modes:

### Automatic
Conver Pay chooses the route from measured performance and health signals.

### Priority
Merchant defines preferred provider order and Conver Pay performs safe failover.

### Rules
Merchant defines explicit routing constraints.

Every routing decision must be auditable.

The UI should expose both:

- current configuration;
- actual observed routing behavior.

Avoid opaque "AI decided" copy.

## 16. Observability

Observability should feel closer to a modern developer observability product than to a finance report.

Required metrics:

- provider availability;
- QR creation latency;
- QR creation success rate;
- payment conversion;
- error classes;
- timeout rate;
- webhook delivery latency;
- routing distribution;
- fallback rate;
- recovery rate;
- recovered revenue.

Charts should support hover inspection, zoom/range selection and drill-down to the payments behind a data point.

## 17. Webhooks and Developer surfaces

Developer surfaces should be crisp and technical.

Include:

- API keys;
- webhook endpoints;
- signing secrets;
- API logs;
- request IDs;
- idempotency keys;
- sandbox/live selector;
- request/response inspector;
- webhook replay;
- copy-as-cURL later;
- documentation links.

Secrets must never leak into logs or normal UI states.

## 18. Interaction and motion

Motion should reinforce cause and effect.

Good uses:

- route switching;
- provider health transition;
- payment status transition;
- inspector opening;
- command palette;
- topology updates;
- recovery confirmation.

Avoid:

- constant ambient animations;
- decorative loading loops;
- bouncing KPI cards;
- excessive spring effects;
- animation that delays access to data.

Perceived speed is part of the brand. Interactions should feel immediate.

## 19. Empty, loading and failure states

These states are part of the product design.

Loading:
- use structural skeletons only when needed;
- prefer streamed/partial data where possible;
- never block unrelated parts of the page.

Failure:
- show exact operational state;
- include error class, timestamp and retry/recovery action;
- do not use vague "something went wrong" messages for provider failures.

Empty:
- explain the next meaningful action;
- avoid generic illustrations.

## 20. Anti-patterns

Do not build:

- a Revolut clone;
- a Modern Treasury clone;
- a Stripe clone;
- a generic shadcn dashboard;
- a legacy banking dashboard;
- a wall of rounded KPI cards;
- arbitrary gradients;
- excessive glassmorphism;
- cyberpunk/neon UI;
- fake real-time animations;
- AI-looking purple gradients everywhere;
- provider rankings without sample/window context;
- recovered-revenue claims without auditable evidence.

## 21. Signature Conver Pay elements

The visual identity should emerge from a small number of recurring, product-specific elements:

1. Live Routing Topology
2. Recovery Event
3. Payment Attempt Timeline
4. Provider Health Node
5. Routing Decision Inspector
6. Global Command Palette
7. Live/Test environment state

These components should receive more design attention than generic cards or marketing decoration.

## 22. Design principle

Conver Pay should look modern because the product model is modern, not because it uses trendy visual effects.

The target experience is:

**Linear-level clarity + Raycast-level finish + Attio-level data interaction + Primer-level payment semantics + Stripe/Vercel-level developer confidence — with a distinct Conver Pay routing identity.**
