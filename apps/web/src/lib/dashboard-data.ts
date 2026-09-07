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
