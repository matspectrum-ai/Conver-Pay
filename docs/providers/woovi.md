# Woovi/OpenPix Provider Capability Matrix

Status: adapter implemented; contract tests green; live sandbox evidence pending.
Last reviewed: 2026-09-07.

## Integration identity

- `provider_key`: `woovi`
- production API: `https://api.woovi.com`
- sandbox API: `https://api.woovi-sandbox.com`
- authentication: raw AppID in `Authorization` header (no Bearer prefix)
- Conver Pay stores the merchant AppID encrypted per `ProviderConnection`.

## Confirmed from current Woovi documentation

| Capability | Endpoint / mechanism | Conver Pay behavior |
| --- | --- | --- |
| Validate AppID | `GET /api/v1/company` | Required before connection is persisted |
| Create Pix charge | `POST /api/v1/charge` | `PaymentAttempt.ID` is sent as `correlationID` |
| Provider idempotency | `correlationID` | Same attempt always reuses the same correlation identity |
| Reconcile charge | `GET /api/v1/charge/{id-or-correlationID}` | Query by `PaymentAttempt.ID` after ambiguous create |
| Paid webhook | `OPENPIX:CHARGE_COMPLETED` | Normalized to `payment.paid` |
| Webhook authenticity | `x-webhook-signature` | RSA-SHA256 verification over exact raw request body |
| Webhook public keys | `GET /api/v1/webhook/public-keys` | Cached; stale verified keys may be retained during fetch failure |
| Public-key rotation | multiple returned keys | Verify against every usable published key |

## Safety classification

Create is effectful. Conver Pay therefore does not infer "not created" from a transport failure or generic HTTP error.

| Observation | Result |
| --- | --- |
| Valid 2xx with charge identifier + BR Code | `succeeded` |
| 401 / 403 | terminal configuration/auth failure |
| Transport timeout/error | `unknown` |
| 429 / 5xx | `unknown` |
| Other ambiguous non-2xx | `unknown` |
| Malformed 2xx | `unknown` |

For reconciliation, a single `404` currently remains `unknown`. We do **not** enable cross-provider fallback based on one negative read because the public documentation does not establish a consistency/propagation guarantee sufficient to prove the charge was never created.

## Sandbox evidence gate

Before marking Phase 4 complete, record observed sandbox evidence for:

1. valid AppID validation;
2. create charge response and actual field shapes;
3. repeated create with identical `correlationID`;
4. lookup by `correlationID` immediately after successful create;
5. lookup behavior after an intentionally interrupted/ambiguous create if reproducible;
6. webhook signature verification using an actual sandbox delivery;
7. duplicate webhook delivery behavior;
8. negative lookup consistency window sufficient to decide whether any `failed_safe` reconciliation policy is defensible.

Until item 8 is proven, Woovi ambiguous creates intentionally block automatic cross-provider failover.
