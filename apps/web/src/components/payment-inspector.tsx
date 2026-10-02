"use client";

import { useEffect, useState } from "react";
import type { Payment } from "@/lib/dashboard-data";
import { getPaymentInspectorDetail } from "@/lib/dashboard-data";

function statusClass(status: string) {
  return `status status-${status.toLowerCase().replaceAll(" ", "-")}`;
}

export function PaymentInspector({ payment, onClose, onNavigate }: { payment: Payment | null; onClose: () => void; onNavigate: (direction: -1 | 1) => void }) {
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    if (!payment) return;
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
      if (event.key === "ArrowUp") { event.preventDefault(); onNavigate(-1); }
      if (event.key === "ArrowDown") { event.preventDefault(); onNavigate(1); }
    };
    window.addEventListener("keydown", onKey);
    return () => {
      document.body.style.overflow = previous;
      window.removeEventListener("keydown", onKey);
    };
  }, [payment, onClose, onNavigate]);

  if (!payment) return null;
  const detail = getPaymentInspectorDetail(payment);
  const recovered = payment.status === "Recovered";
  const reconciling = payment.status === "Reconciling";

  const copyPix = async () => {
    await navigator.clipboard.writeText(detail.pixCode);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1400);
  };

  return (
    <div className="inspector-layer">
      <button className="inspector-scrim" aria-label="Close payment inspector" onClick={onClose} />
      <aside className="payment-inspector" role="dialog" aria-modal="true" aria-labelledby="payment-inspector-title">
        <header className="inspector-header">
          <div className="inspector-title-wrap">
            <span className="inspector-kicker">PAYMENT</span>
            <div className="inspector-title-line"><h2 id="payment-inspector-title">{payment.id}</h2><span className={statusClass(payment.status)}><i />{payment.status}</span></div>
            <span>{payment.order}</span>
          </div>
          <button className="inspector-close" onClick={onClose} aria-label="Close inspector">×<kbd>Esc</kbd></button>
        </header>

        <div className="inspector-body">
          <section className="inspector-amount-row"><div><span>Amount</span><strong>{payment.amount}</strong></div><div><span>Environment</span><strong><i className="live-dot" /> Live</strong></div></section>

          {recovered && <section className="inspector-callout recovered"><div className="callout-icon">↗</div><div><span>RECOVERED BY CONVER</span><strong>{payment.amount} protected</strong><p>Primary failed safely. Fallback created a usable Pix that was later paid.</p></div></section>}
          {reconciling && <section className="inspector-callout warning"><div className="callout-icon">!</div><div><span>DUPLICATE PIX PROTECTION</span><strong>Cross-provider fallback blocked</strong><p>Create outcome is ambiguous. Conver is reconciling before another provider can be attempted.</p></div></section>}

          <section className="inspector-section">
            <div className="inspector-section-heading"><div><span>PAYMENT STATE</span><h3>Canonical details</h3></div></div>
            <dl className="detail-grid">
              <div><dt>Selected provider</dt><dd>{detail.routing.selected}</dd></div><div><dt>QR latency</dt><dd>{payment.latency}</dd></div>
              <div><dt>Provider payment ID</dt><dd className="mono-value">{detail.providerPaymentId}</dd></div><div><dt>Currency</dt><dd>{detail.currency}</dd></div>
              <div><dt>Created</dt><dd>{detail.createdAt}</dd></div><div><dt>Last update</dt><dd>{detail.updatedAt}</dd></div>
            </dl>
          </section>

          {!reconciling && <section className="inspector-section"><div className="inspector-section-heading"><div><span>PIX</span><h3>Copy & paste</h3></div><button className="inspector-action" onClick={copyPix}>{copied ? "Copied" : "Copy code"}</button></div><div className="pix-code"><code>{detail.pixCode}</code></div></section>}

          <section className="inspector-section">
            <div className="inspector-section-heading"><div><span>ROUTING DECISION</span><h3>{detail.routing.policy}</h3></div><span className="decision-id">rd_{payment.id.slice(3)}</span></div>
            <p className="routing-explanation">{detail.routing.reason}</p>
            <div className="candidate-list">{detail.routing.candidates.map((candidate) => <div className="candidate-row" key={candidate.provider}><div><span className={`health-dot ${candidate.eligible ? "healthy" : "degraded"}`} /><strong>{candidate.provider}</strong>{!candidate.eligible && <em>ineligible</em>}</div><div className="score-track"><i style={{ width: `${candidate.score * 100}%` }} /></div><b>{Math.round(candidate.score * 100)}</b></div>)}</div>
          </section>

          <section className="inspector-section">
            <div className="inspector-section-heading"><div><span>ATTEMPTS</span><h3>{detail.attempts.length} provider {detail.attempts.length === 1 ? "attempt" : "attempts"}</h3></div></div>
            <div className="attempt-list">{detail.attempts.map((attempt) => <div className="attempt-row" key={`${attempt.provider}-${attempt.index}`}><span className={`attempt-index ${attempt.status}`}>{String(attempt.index).padStart(2, "0")}</span><div className="attempt-main"><div><strong>{attempt.provider}</strong><span>{attempt.latency}</span></div><p>{attempt.detail}</p><code>{attempt.providerPaymentId}</code></div></div>)}</div>
          </section>

          <section className="inspector-section timeline-section">
            <div className="inspector-section-heading"><div><span>TECHNICAL TIMELINE</span><h3>Event evidence</h3></div><span className="decision-id">{detail.requestId}</span></div>
            <div className="inspector-timeline">{detail.timeline.map((event, index) => <div className={`timeline-event ${event.tone ?? ""}`} key={`${event.time}-${index}`}><div className="timeline-rail"><i />{index < detail.timeline.length - 1 && <span />}</div><time>{event.time}</time><div><span>{event.actor}</span><strong>{event.label}</strong>{event.detail && <p>{event.detail}</p>}</div></div>)}</div>
          </section>

          <section className="inspector-section raw-section"><div className="inspector-section-heading"><div><span>TECHNICAL</span><h3>Identifiers</h3></div></div><div className="raw-identifiers"><div><span>payment_intent</span><code>{payment.id}</code></div><div><span>merchant_order_id</span><code>{payment.order}</code></div><div><span>request_id</span><code>{detail.requestId}</code></div></div></section>
        </div>

        <footer className="inspector-footer"><span>Payment Inspector</span><div><kbd>↑</kbd><kbd>↓</kbd><span>Navigate</span><kbd>Esc</kbd><span>Close</span></div></footer>
      </aside>
    </div>
  );
}
