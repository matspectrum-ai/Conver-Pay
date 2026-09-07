export const providers = [
  { id: "woovi", name: "Woovi", share: 48, latency: 184, success: 99.4, score: 0.94, state: "healthy" as const },
  { id: "pagarme", name: "Pagar.me", share: 34, latency: 231, success: 98.9, score: 0.89, state: "healthy" as const },
  { id: "blackcat", name: "BlackCat", share: 18, latency: 612, success: 94.8, score: 0.63, state: "degraded" as const },
];

export const recoveries = [
  { id: "rec_4821", amount: "R$ 197,00", from: "BlackCat", to: "Woovi", age: "18s", reason: "503 upstream" },
  { id: "rec_4820", amount: "R$ 49,90", from: "Pagar.me", to: "Woovi", age: "41s", reason: "hard failure" },
  { id: "rec_4819", amount: "R$ 129,00", from: "BlackCat", to: "Pagar.me", age: "1m", reason: "circuit open" },
];

export const payments = [
  { id: "pi_8hf72", order: "order_19281", amount: "R$ 500,00", provider: "Woovi", status: "Paid", latency: "184 ms", age: "12s" },
  { id: "pi_1k0p9", order: "order_19280", amount: "R$ 197,00", provider: "Woovi", status: "Recovered", latency: "201 ms", age: "18s" },
  { id: "pi_4n2da", order: "order_19279", amount: "R$ 89,90", provider: "Pagar.me", status: "Awaiting", latency: "228 ms", age: "25s" },
  { id: "pi_0z88e", order: "order_19278", amount: "R$ 1.249,00", provider: "Woovi", status: "Paid", latency: "172 ms", age: "39s" },
  { id: "pi_772bx", order: "order_19277", amount: "R$ 59,00", provider: "BlackCat", status: "Reconciling", latency: "—", age: "52s" },
  { id: "pi_3c91q", order: "order_19276", amount: "R$ 349,90", provider: "Pagar.me", status: "Paid", latency: "244 ms", age: "1m" },
];

export type Payment = (typeof payments)[number];

type Candidate = { provider: string; score: number; eligible: boolean };
type Attempt = { index: number; provider: string; status: "succeeded" | "failed_safe" | "unknown"; latency: string; providerPaymentId: string; detail: string };
type TimelineEvent = { time: string; actor: string; label: string; tone?: "success" | "danger" | "warning"; detail?: string };

export type PaymentInspectorDetail = {
  createdAt: string;
  updatedAt: string;
  currency: string;
  pixCode: string;
  providerPaymentId: string;
  requestId: string;
  routing: { policy: string; selected: string; reason: string; candidates: Candidate[] };
  attempts: Attempt[];
  timeline: TimelineEvent[];
};

const pixCode = "00020126580014BR.GOV.BCB.PIX0136conver-pay-demo-pix-key-00005204000053039865406500.005802BR5912CONVER DEMO6009SAO PAULO62070503***6304A1B2";
const baseCandidates: Candidate[] = [
  { provider: "Woovi", score: 0.94, eligible: true },
  { provider: "Pagar.me", score: 0.89, eligible: true },
  { provider: "BlackCat", score: 0.63, eligible: true },
];

export function getPaymentInspectorDetail(payment: Payment): PaymentInspectorDetail {
  const base = {
    createdAt: "Sep 7, 2026 · 17:18:42.117",
    updatedAt: "Sep 7, 2026 · 17:18:55.819",
    currency: "BRL",
    pixCode,
    providerPaymentId: `px_${payment.id.slice(3)}_01`,
    requestId: `req_${payment.id.slice(3)}_7f1a`,
  };

  if (payment.status === "Recovered") {
    return {
      ...base,
      routing: {
        policy: "Automatic · health-v1",
        selected: "Woovi",
        reason: "BlackCat returned a verifiable 503. Cross-provider fallback was safe and Woovi was the highest-scoring eligible candidate.",
        candidates: [baseCandidates[0], baseCandidates[1], { ...baseCandidates[2], eligible: false }],
      },
      attempts: [
        { index: 1, provider: "BlackCat", status: "failed_safe", latency: "112 ms", providerPaymentId: "bc_82721", detail: "503 SERVICE_UNAVAILABLE · safe to fallback" },
        { index: 2, provider: "Woovi", status: "succeeded", latency: "201 ms", providerPaymentId: "opx_99128", detail: "PIX_CREATED · selected attempt" },
      ],
      timeline: [
        { time: "17:18:42.117", actor: "Conver", label: "Payment intent created", detail: "Idempotency key order_19280" },
        { time: "17:18:42.184", actor: "BlackCat", label: "503 SERVICE_UNAVAILABLE", tone: "danger", detail: "Hard provider failure; no charge created" },
        { time: "17:18:42.226", actor: "Conver", label: "Fallback triggered", tone: "warning", detail: "health-v1 selected Woovi · score 0.94" },
        { time: "17:18:42.401", actor: "Woovi", label: "PIX_CREATED", detail: "QR available in 201 ms" },
        { time: "17:18:55.819", actor: "Pix", label: `PAID · ${payment.amount}`, tone: "success", detail: "Recovery evidence sealed" },
      ],
    };
  }

  if (payment.status === "Reconciling") {
    return {
      ...base,
      providerPaymentId: "—",
      routing: {
        policy: "Automatic · health-v1",
        selected: "BlackCat",
        reason: "The create request timed out after transmission. Outcome is ambiguous, so cross-provider fallback is blocked until reconciliation proves no charge exists.",
        candidates: baseCandidates,
      },
      attempts: [{ index: 1, provider: "BlackCat", status: "unknown", latency: "5,000 ms", providerPaymentId: "unknown", detail: "Transport timeout · create outcome ambiguous" }],
      timeline: [
        { time: "17:18:42.117", actor: "Conver", label: "Payment intent created" },
        { time: "17:18:47.118", actor: "BlackCat", label: "Create outcome unknown", tone: "warning", detail: "Client timeout after request transmission" },
        { time: "17:18:47.120", actor: "Conver", label: "Cross-provider fallback blocked", tone: "warning", detail: "Duplicate Pix protection" },
        { time: "17:18:47.401", actor: "Worker", label: "Reconciliation scheduled", detail: "Provider lookup pending" },
      ],
    };
  }

  return {
    ...base,
    routing: {
      policy: "Automatic · health-v1",
      selected: payment.provider,
      reason: `${payment.provider} was the highest-scoring eligible provider at decision time. No fallback was required.`,
      candidates: baseCandidates,
    },
    attempts: [{ index: 1, provider: payment.provider, status: "succeeded", latency: payment.latency, providerPaymentId: `px_${payment.id.slice(3)}_01`, detail: "PIX_CREATED · selected attempt" }],
    timeline: [
      { time: "17:18:42.117", actor: "Conver", label: "Payment intent created", detail: `Order ${payment.order}` },
      { time: "17:18:42.153", actor: "Router", label: `${payment.provider} selected`, detail: "Automatic · health-v1" },
      { time: "17:18:42.401", actor: payment.provider, label: "PIX_CREATED", detail: `QR available in ${payment.latency}` },
      ...(payment.status === "Paid" ? [{ time: "17:18:55.819", actor: "Pix", label: `PAID · ${payment.amount}`, tone: "success" as const }] : []),
    ],
  };
}
