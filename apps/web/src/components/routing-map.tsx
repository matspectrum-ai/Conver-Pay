import { providers } from "@/lib/dashboard-data";

export function RoutingMap() {
  return (
    <div className="routing-map" role="img" aria-label="Live routing topology">
      <div className="routing-grid" />
      <svg className="routing-lines" viewBox="0 0 760 330" preserveAspectRatio="none" aria-hidden="true">
        <defs>
          <linearGradient id="routeHealthy" x1="0" x2="1">
            <stop offset="0" stopColor="#434345" stopOpacity=".55" />
            <stop offset="1" stopColor="#59d499" stopOpacity=".85" />
          </linearGradient>
          <linearGradient id="routeWarn" x1="0" x2="1">
            <stop offset="0" stopColor="#434345" stopOpacity=".45" />
            <stop offset="1" stopColor="#ffc533" stopOpacity=".8" />
          </linearGradient>
        </defs>
        <path d="M380 109 C300 135 237 174 170 235" stroke="url(#routeHealthy)" />
        <path d="M380 109 C380 153 380 187 380 235" stroke="url(#routeHealthy)" />
        <path d="M380 109 C460 135 523 174 590 235" stroke="url(#routeWarn)" />
      </svg>
      <div className="router-core">
        <div className="router-ring"><span className="brand-glyph mini"><i /><b /></span></div>
        <span>Conver</span>
        <small>automatic</small>
      </div>
      {providers.map((provider, index) => (
        <div className={`provider-node provider-node-${index + 1}`} key={provider.id}>
          <div className={`node-dot ${provider.state}`} />
          <div>
            <strong>{provider.name}</strong>
            <span>{provider.share}% traffic</span>
          </div>
        </div>
      ))}
      <div className="routing-live-badge"><span className="live-dot" /> live</div>
      <div className="routing-caption">Routing responds to rolling health snapshots, not static priority.</div>
    </div>
  );
}
