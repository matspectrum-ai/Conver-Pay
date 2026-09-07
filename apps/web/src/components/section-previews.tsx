import { Icons } from "./icons";
import { RoutingMap } from "./routing-map";
import { payments, providers, recoveries } from "@/lib/dashboard-data";

function PageHeader({ kicker, title, copy }: { kicker: string; title: string; copy: string }) {
  return <section className="subpage-header"><div><p className="eyebrow">{kicker}</p><h1>{title}</h1><p>{copy}</p></div></section>;
}

function MiniMetric({ label, value, detail, tone }: { label: string; value: string; detail: string; tone?: string }) {
  return <div className="metric"><span>{label}</span><strong className={tone === "green" ? "text-green" : ""}>{value}</strong><small>{detail}</small></div>;
}

function PaymentsView() {
  return <>
    <PageHeader kicker="PAYMENTS" title="Payments" copy="Every Pix intent, provider attempt and canonical state in one operational view." />
    <section className="metric-strip subpage-metrics"><MiniMetric label="Created" value="11,083" detail="Last 7 days"/><MiniMetric label="Paid" value="8,491" detail="76.8% conversion"/><MiniMetric label="Reconciling" value="7" detail="0.06% of volume"/><MiniMetric label="p95 QR latency" value="418 ms" detail="Across all providers"/></section>
    <div className="section-toolbar"><div className="inline-search"><Icons.search className="icon"/>Search payment, order or provider…<kbd>/</kbd></div><div><button className="ghost-button">Status</button><button className="ghost-button">Provider</button><button className="ghost-button">Date</button></div></div>
    <div className="table-wrap"><table><thead><tr><th>Payment</th><th>Order</th><th>Amount</th><th>Provider</th><th>Status</th><th>QR latency</th><th>Created</th></tr></thead><tbody>{payments.concat(payments.slice(0,2)).map((payment,index)=><tr key={payment.id+index}><td><button className="mono-link">{payment.id}</button></td><td className="muted-cell">{payment.order}</td><td className="amount-cell">{payment.amount}</td><td>{payment.provider}</td><td><span className={`status status-${payment.status.toLowerCase()}`}><i/>{payment.status}</span></td><td className="muted-cell">{payment.latency}</td><td className="muted-cell">{payment.age}</td></tr>)}</tbody></table></div>
  </>;
}

function RecoveriesView() {
  return <>
    <PageHeader kicker="RECOVERED REVENUE" title="Recoveries" copy="Only payments with auditable primary failure, fallback and successful settlement are attributed here." />
    <section className="metric-strip subpage-metrics"><MiniMetric label="Recovered revenue" value="R$ 31.892" detail="Last 7 days" tone="green"/><MiniMetric label="Recovered payments" value="184" detail="1.66% of intents"/><MiniMetric label="Median recovery" value="1.8s" detail="Failure → fallback QR"/><MiniMetric label="Top recovery path" value="BlackCat → Woovi" detail="R$ 18.204 protected"/></section>
    <section className="two-column-detail"><div className="panel"><div className="panel-header compact"><div><span className="section-kicker">LATEST</span><h2>Recovery evidence</h2></div><span className="live-label"><span className="live-dot"/>live</span></div><div className="recovery-list expanded">{recoveries.concat(recoveries).map((r,i)=><div className="recovery-row" key={r.id+i}><div className="recovery-icon">↗</div><div className="recovery-main"><strong>{r.amount}</strong><span>{r.from} <b>→</b> {r.to}</span></div><span className="recovery-reason">{r.reason}</span><time>{i<3?r.age:`${i+1}m`}</time></div>)}</div></div><div className="panel evidence-panel"><span className="section-kicker">EVIDENCE SAMPLE</span><h2>rec_4821</h2><div className="evidence-timeline"><div className="failed"><i/><span>20:31:02.117</span><strong>BlackCat</strong><b>503 SERVICE_UNAVAILABLE</b></div><div><i/><span>20:31:02.184</span><strong>Conver</strong><b>Failover triggered</b></div><div><i/><span>20:31:02.401</span><strong>Woovi</strong><b>PIX_CREATED · 201ms</b></div><div className="success"><i/><span>20:33:11.819</span><strong>Pix</strong><b>PAID · R$ 197,00</b></div></div><button className="ghost-button">Open full payment <Icons.arrow className="icon"/></button></div></section>
  </>;
}

function RoutingView() {
  return <>
    <PageHeader kicker="AUTOMATIC ROUTING" title="Routing" copy="Deterministic provider selection using rolling health, eligibility and auditable score snapshots." />
    <section className="routing-page-grid"><div className="panel"><div className="panel-header"><div><span className="section-kicker">CURRENT FLOW</span><h2>Live topology</h2></div><span className="live-label"><span className="live-dot"/>live</span></div><RoutingMap/></div><div className="panel rules-panel"><div className="panel-header compact"><div><span className="section-kicker">POLICY</span><h2>Automatic · health-v1</h2></div><button className="ghost-button">Edit</button></div><div className="setting-list"><div><span>QR creation success</span><strong>50%</strong></div><div><span>Inverse error rate</span><strong>20%</strong></div><div><span>Inverse timeout rate</span><strong>15%</strong></div><div><span>p95 latency</span><strong>15%</strong></div></div><div className="policy-note"><span className="health-dot healthy"/><p>Cold start score is neutral. Static priority is used only as a tie-breaker.</p></div></div></section>
  </>;
}

function ProvidersView() {
  return <>
    <PageHeader kicker="CONNECTIONS" title="Providers" copy="Merchant-owned provider connections, health and traffic allocation." />
    <div className="provider-directory">{providers.map((provider)=><div className="provider-directory-row" key={provider.id}><div className="provider-identity"><span className={`provider-logo provider-${provider.id}`}>{provider.name[0]}</span><div><strong>{provider.name}</strong><span>Live · credentials verified</span></div></div><div className="provider-directory-stat"><span>Traffic</span><strong>{provider.share}%</strong></div><div className="provider-directory-stat"><span>QR success</span><strong>{provider.success}%</strong></div><div className="provider-directory-stat"><span>p95 latency</span><strong>{provider.latency} ms</strong></div><div className="provider-directory-stat"><span>Health score</span><strong>{Math.round(provider.score*100)}</strong></div><span className={`status ${provider.state==='healthy'?'status-paid':'status-reconciling'}`}><i/>{provider.state}</span><button className="row-action">›</button></div>)}</div>
  </>;
}

function ObservabilityView() {
  return <>
    <PageHeader kicker="OBSERVABILITY" title="Network health" copy="Provider-level creation, latency, error and webhook signals without high-cardinality noise." />
    <section className="metric-strip subpage-metrics"><MiniMetric label="Provider availability" value="99.92%" detail="Weighted, 24h"/><MiniMetric label="QR creation success" value="99.16%" detail="Across 11,083 creates"/><MiniMetric label="p50 / p95 latency" value="192 / 418ms" detail="Create response"/><MiniMetric label="Webhook delivery" value="99.98%" detail="Merchant outbound"/></section>
    <section className="observability-grid"><div className="panel chart-panel"><div className="panel-header compact"><div><span className="section-kicker">LATENCY</span><h2>QR creation p95</h2></div><span className="muted">Last 6 hours</span></div><div className="line-chart"><div className="chart-y"><span>800ms</span><span>400ms</span><span>0</span></div><svg viewBox="0 0 640 190" preserveAspectRatio="none"><path className="chart-line green" d="M0 138C50 125 80 142 120 116S190 102 230 112 300 89 345 98 420 72 460 83 535 70 640 60"/><path className="chart-line blue" d="M0 113C60 98 95 105 135 87S220 95 260 82 340 79 390 74 470 65 520 69 590 52 640 56"/><path className="chart-line yellow" d="M0 92C55 89 110 82 150 96S235 68 280 79 350 58 395 70 465 38 520 46 585 29 640 20"/></svg><div className="chart-legend"><span><i className="legend-green"/>Woovi</span><span><i className="legend-blue"/>Pagar.me</span><span><i className="legend-yellow"/>BlackCat</span></div></div></div><div className="panel"><div className="panel-header compact"><div><span className="section-kicker">ERRORS</span><h2>Failure taxonomy</h2></div></div><div className="error-bars"><div><span>Provider 5xx</span><b><i style={{width:'46%'}}/></b><strong>46</strong></div><div><span>Timeout</span><b><i style={{width:'24%'}}/></b><strong>24</strong></div><div><span>Auth / config</span><b><i style={{width:'7%'}}/></b><strong>7</strong></div><div><span>Unknown</span><b><i style={{width:'3%'}}/></b><strong>3</strong></div></div></div></section>
  </>;
}

function WebhooksView() {
  const deliveries=[['evt_91fk2','payment.paid','200','184ms','12s'],['evt_91fjy','payment.recovered','200','201ms','18s'],['evt_91fhu','payment.paid','200','163ms','39s'],['evt_91fgt','payment.paid','429 → retry','—','52s']];
  return <><PageHeader kicker="WEBHOOKS" title="Webhooks" copy="Provider ingress and signed merchant delivery are isolated, observable flows."/><section className="endpoint-panel panel"><div><span className="section-kicker">MERCHANT OUTBOUND</span><h2>https://api.example.com/webhooks/conver-pay</h2><p>HMAC-SHA256 · Live environment</p></div><span className="status status-paid"><i/>Active</span></section><div className="payments-heading"><div><span className="section-kicker">DELIVERIES</span><h2>Recent events</h2></div><button className="ghost-button">Delivery logs <Icons.arrow className="icon"/></button></div><div className="table-wrap"><table><thead><tr><th>Event</th><th>Type</th><th>Result</th><th>Latency</th><th>Age</th></tr></thead><tbody>{deliveries.map(d=><tr key={d[0]}><td><button className="mono-link">{d[0]}</button></td><td>{d[1]}</td><td className={d[2].startsWith('200')?'text-green':'text-yellow'}>{d[2]}</td><td className="muted-cell">{d[3]}</td><td className="muted-cell">{d[4]}</td></tr>)}</tbody></table></div></>;
}

function DevelopersView() {
  return <><PageHeader kicker="DEVELOPERS" title="API & keys" copy="One provider-neutral Pix contract across every connected downstream provider."/><section className="developer-grid"><div className="panel api-key-panel"><div className="panel-header compact"><div><span className="section-kicker">API KEYS</span><h2>Live keys</h2></div></div><div className="key-row"><div><strong>Production</strong><span>cp_live_••••••••••••4k2f</span></div><span>Last used 12s ago</span><button className="ghost-button">•••</button></div><div className="key-row"><div><strong>Backend sync</strong><span>cp_live_••••••••••••81xm</span></div><span>Last used 3h ago</span><button className="ghost-button">•••</button></div></div><div className="panel quickstart"><span className="section-kicker">QUICKSTART</span><h2>Create a Pix payment</h2><pre><code>{`curl -X POST https://api.converpay.com/v1/payment_intents \
  -H "Authorization: Bearer cp_live_..." \
  -H "Idempotency-Key: order_19281" \
  -d '{"merchant_order_id":"order_19281","amount":5000,"currency":"BRL"}'`}</code></pre><button className="ghost-button">Open API reference <Icons.arrow className="icon"/></button></div></section></>;
}

function SettingsView() {
  return <><PageHeader kicker="WORKSPACE" title="Settings" copy="Workspace identity and environment controls. Secrets remain write-only."/><section className="settings-stack"><div className="panel settings-group"><div className="panel-header compact"><div><span className="section-kicker">GENERAL</span><h2>Workspace</h2></div></div><div className="settings-row"><div><strong>Name</strong><span>Shown across the Conver Pay workspace.</span></div><button className="field-button">Conver Pay</button></div><div className="settings-row"><div><strong>Environment</strong><span>Current operational context.</span></div><button className="field-button"><span className="live-dot"/> Live</button></div></div><div className="panel settings-group"><div className="panel-header compact"><div><span className="section-kicker">DANGER ZONE</span><h2>Operational controls</h2></div></div><div className="settings-row"><div><strong>Provider kill switch</strong><span>Immediately stop new routing while preserving reconciliation.</span></div><button className="danger-button">Disable routing</button></div></div></section></>;
}

export function SectionPreview({ section }: { section: string }) {
  switch(section){
    case 'Payments': return <PaymentsView/>;
    case 'Recoveries': return <RecoveriesView/>;
    case 'Routing': return <RoutingView/>;
    case 'Providers': return <ProvidersView/>;
    case 'Observability': return <ObservabilityView/>;
    case 'Webhooks': return <WebhooksView/>;
    case 'Developers': return <DevelopersView/>;
    case 'Settings': return <SettingsView/>;
    default: return null;
  }
}
