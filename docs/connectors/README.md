# Conver Pay Provider Connectors

Provider connectors are the only modules allowed to understand a downstream payment provider's proprietary API.

## Contract

A connector exposes a normalized data-plane contract and a versioned manifest:

- create supported operations;
- reconciliation and lookup behavior;
- webhook verification/normalization;
- capabilities;
- credential types;
- adapter/connector version.

A connector is instantiated from a `ProviderConnection`, so merchant credentials are always connection-scoped.

## Lifecycle

```text
draft → contract-tested → sandbox-validated → production-qualified → deprecated
```

The router may only use enabled, production-qualified connections for live traffic.

## Connector #001

`woovi` is the first connector implementation. It is an implementation example, not a special case in the orchestration core.
