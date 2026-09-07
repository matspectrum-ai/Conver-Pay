"use client";

import { useEffect, useState } from "react";
import { CommandPalette } from "./command-palette";
import { Icons } from "./icons";
import { RoutingMap } from "./routing-map";
import { SectionPreview } from "./section-previews";
import { payments, providers, recoveries } from "@/lib/dashboard-data";

const nav = [
  ["Overview", Icons.overview], ["Payments", Icons.payments], ["Recoveries", Icons.recoveries],
  ["Routing", Icons.routing], ["Providers", Icons.providers], ["Observability", Icons.observe],
  ["Webhooks", Icons.webhooks], ["Developers", Icons.developers], ["Settings", Icons.settings],
] as const;

function Status({ status }: { status: string }) {
  const key = status.toLowerCase().replaceAll(" ", "-");
  return <span className={`status status-${key}`}><i />{status}</span>;
}

function Sparkline() {
  return (
    <svg className="sparkline" viewBox="0 0 170 42" preserveAspectRatio="none" aria-hidden="true">
      <path className="spark-area" d="M0 34L14 31L27 33L42 24L56 27L71 18L86 21L100 14L114 17L128 11L143 14L157 8L170 10L170 42L0 42Z" />
      <path className="spark-stroke" d="M0 34L14 31L27 33L42 24L56 27L71 18L86 21L100 14L114 17L128 11L143 14L157 8L170 10" />
    </svg>
  );
}

export function DashboardShell() {
  const [commandOpen, setCommandOpen] = useState(false);
  const [active, setActive] = useState("Overview");
  const [mobileNav, setMobileNav] = useState(false);
  const primaryAction = active === "Providers" ? "Connect provider"
    : active === "Webhooks" ? "Configure endpoint"
    : active === "Developers" ? "Create API key"
    : "Create payment";

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setCommandOpen((value) => !value);
      }
      if (event.key === "Escape") setCommandOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return (
    <div className="app-frame">
      <aside className={mobileNav ? "sidebar open" : "sidebar"}>
        <div className="workspace-switcher">
          <span className="brand-mark"><span className="brand-glyph"><i /><b /></span></span>
          <div><strong>Conver Pay</strong><span>Production</span></div>
          <Icons.chevron className="icon small" />
        </div>

        <button className="search-trigger" onClick={() => setCommandOpen(true)}>
          <span><Icons.search className="icon" /> Search</span><kbd>⌘ K</kbd>
        </button>

        <nav className="primary-navigation" aria-label="Primary navigation">
          {nav.map(([label, Icon]) => (
            <button key={label} className={active === label ? "nav-item active" : "nav-item"} onClick={() => { setActive(label); setMobileNav(false); }}>
              <Icon className="icon" /><span>{label}</span>
              {label === "Providers" && <i className="attention-dot" />}
            </button>
          ))}
        </nav>

        <div className="sidebar-bottom">
          <div className="environment-row"><span className="live-dot" /> Live environment <kbd>L</kbd></div>
          <div className="profile-row"><span className="avatar">MS</span><div><strong>matspectrum</strong><span>Owner</span></div><span className="more">•••</span></div>
        </div>
      </aside>

      <main className="content-shell">
        <header className="topbar">
          <button className="mobile-menu" onClick={() => setMobileNav((value) => !value)}>☰</button>
          <div className="page-title"><span>Workspace</span><b>/</b><strong>{active}</strong></div>
          <div className="topbar-actions">
            <div className="system-health"><span className="live-dot" /> All systems operational</div>
            <button className="icon-button" aria-label="Notifications">◌<span className="notification-dot" /></button>
            <button className="primary-button"><Icons.plus className="icon" /> {primaryAction}</button>
          </div>
        </header>

        <div className="page-content">
          {active === "Overview" ? <>
          <section className="hero-row">
            <div>
              <p className="eyebrow">Sunday, September 7</p>
              <h1>Payment network is healthy.</h1>
              <p className="hero-copy">Conver is automatically routing Pix traffic across 3 connected providers.</p>
            </div>
            <div className="period-switcher"><button>24h</button><button className="active">7d</button><button>30d</button></div>
          </section>

          <section className="metric-strip" aria-label="Key metrics">
            <div className="metric"><span>Processed volume</span><strong>R$ 487.291</strong><small><b>↑ 18.4%</b> vs previous period</small></div>
            <div className="metric"><span>Pix conversion</span><strong>76.8%</strong><small><b>↑ 2.1%</b> from 74.7%</small></div>
            <div className="metric recovered-metric"><span>Recovered revenue</span><strong>R$ 31.892</strong><small><b>+ R$ 3.941</b> protected today</small></div>
            <div className="metric graph-metric"><span>Successful payments</span><strong>8,491</strong><Sparkline /></div>
          </section>

          <section className="operations-grid">
            <div className="panel routing-panel">
              <div className="panel-header">
                <div><span className="section-kicker">AUTOMATIC ROUTING</span><h2>Live payment flow</h2></div>
                <button className="ghost-button">Routing details <Icons.arrow className="icon" /></button>
              </div>
              <RoutingMap />
            </div>

            <div className="panel provider-panel">
              <div className="panel-header compact"><div><span className="section-kicker">PROVIDERS</span><h2>Health</h2></div><span className="muted">15m window</span></div>
              <div className="provider-list">
                {providers.map((provider) => (
                  <div className="provider-row" key={provider.id}>
                    <div className="provider-identity"><span className={`provider-logo provider-${provider.id}`}>{provider.name.slice(0,1)}</span><div><strong>{provider.name}</strong><span>{provider.share}% traffic</span></div></div>
                    <div className="provider-score"><strong>{Math.round(provider.score * 100)}</strong><span>score</span></div>
                    <div className="provider-meta"><span>{provider.latency} ms</span><span>{provider.success}% QR</span></div>
                    <span className={`health-dot ${provider.state}`} />
                  </div>
                ))}
              </div>
              <button className="panel-footer-link">View provider observability <Icons.arrow className="icon" /></button>
            </div>
          </section>

          <section className="secondary-grid">
            <div className="panel recovery-panel">
              <div className="panel-header compact"><div><span className="section-kicker">RECOVERIES</span><h2>Revenue protected</h2></div><span className="live-label"><span className="live-dot" /> live</span></div>
              <div className="recovery-list">
                {recoveries.map((recovery) => (
                  <div className="recovery-row" key={recovery.id}>
                    <div className="recovery-icon">↗</div>
                    <div className="recovery-main"><strong>{recovery.amount}</strong><span>{recovery.from} <b>→</b> {recovery.to}</span></div>
                    <span className="recovery-reason">{recovery.reason}</span><time>{recovery.age}</time>
                  </div>
                ))}
              </div>
            </div>

            <div className="panel insight-panel">
              <div className="insight-orb"><span /><span /><span /></div>
              <span className="section-kicker">ROUTING INSIGHT</span>
              <h2>BlackCat is degrading</h2>
              <p>p95 latency rose 63% in the last 15 minutes. Traffic was reduced from 31% to 18% automatically.</p>
              <button className="ghost-button">Inspect decision <Icons.arrow className="icon" /></button>
            </div>
          </section>

          <section className="payments-section">
            <div className="payments-heading"><div><span className="section-kicker">PAYMENTS</span><h2>Recent activity</h2></div><div className="table-actions"><button className="ghost-button">Filters <span className="filter-count">2</span></button><button className="ghost-button">View all <Icons.arrow className="icon" /></button></div></div>
            <div className="table-wrap">
              <table>
                <thead><tr><th>Payment</th><th>Order</th><th>Amount</th><th>Provider</th><th>Status</th><th>QR latency</th><th></th></tr></thead>
                <tbody>{payments.map((payment) => (
                  <tr key={payment.id}><td><button className="mono-link">{payment.id}</button><span className="row-age">{payment.age}</span></td><td className="muted-cell">{payment.order}</td><td className="amount-cell">{payment.amount}</td><td><span className="provider-inline"><i />{payment.provider}</span></td><td><Status status={payment.status} /></td><td className="muted-cell">{payment.latency}</td><td><button className="row-action">›</button></td></tr>
                ))}</tbody>
              </table>
            </div>
          </section>
          </> : <SectionPreview section={active} />}
        </div>
      </main>
      {mobileNav && <button className="mobile-scrim" aria-label="Close navigation" onClick={() => setMobileNav(false)} />}
      <CommandPalette open={commandOpen} onClose={() => setCommandOpen(false)} />
    </div>
  );
}
