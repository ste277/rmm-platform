import { useEffect, useState, useCallback } from "react";

type AlertRule = {
  id: string;
  name: string;
  metric_name: string;
  operator: string;
  threshold: number;
  duration_seconds: number;
  enabled: boolean;
  created_at?: string;
};

type AlertEvent = {
  id: number;
  rule_id: string;
  rule_name: string;
  agent_id: string;
  metric_value: number;
  fired_at: string;
  resolved_at?: string;
};

const GATEWAY = "http://127.0.0.1:8080";

const METRICS = [
  "system.cpu.percent",
  "system.mem.percent",
  "system.disk.percent",
  "system.mem.used_bytes",
  "system.disk.used_bytes",
];

export function AlertsPage() {
  const [rules, setRules] = useState<AlertRule[]>([]);
  const [events, setEvents] = useState<AlertEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const [name, setName] = useState("");
  const [metric, setMetric] = useState(METRICS[0]);
  const [operator, setOperator] = useState(">");
  const [threshold, setThreshold] = useState("80");

  const load = useCallback(async () => {
    try {
      const [r, e] = await Promise.all([
        fetch(`${GATEWAY}/api/v1/alert-rules?tenant_id=dev-tenant-001`).then(r => r.json()),
        fetch(`${GATEWAY}/api/v1/alert-events?tenant_id=dev-tenant-001`).then(r => r.json()),
      ]);
      setRules(r ?? []);
      setEvents(e ?? []);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "failed to load");
    }
  }, []);

  useEffect(() => {
    load();
    const t = setInterval(load, 30000);
    return () => clearInterval(t);
  }, [load]);

  async function handleCreate() {
    if (!name) return;
    setError(null);
    try {
      const r = await fetch(`${GATEWAY}/api/v1/alert-rules`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          tenant_id: "dev-tenant-001",
          name,
          metric_name: metric,
          operator,
          threshold: parseFloat(threshold),
          duration_seconds: 0,
          enabled: true,
        }),
      });
      if (!r.ok) throw new Error(await r.text());
      setName(""); setThreshold("80");
      setSuccess("Alert rule created");
      await load();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "create failed");
    }
  }

  async function handleDelete(id: string) {
    try {
      await fetch(`${GATEWAY}/api/v1/alert-rules/${id}`, { method: "DELETE" });
      await load();
    } catch {}
  }

  const firing = events.filter(e => !e.resolved_at).length;

  return (
    <main className="page">
      <header className="page-header">
        <h1 className="page-title">Alerts</h1>
        <div className="page-meta">
          <span className="meta-badge">{rules.length} rules</span>
          {firing > 0 && <span className="meta-badge danger">{firing} firing</span>}
          <button className="refresh-btn" onClick={load}>↺ Refresh</button>
        </div>
      </header>

      {error && <div className="error-bar">{error}</div>}
      {success && <div className="success-bar">{success}</div>}

      {/* New rule */}
      <section className="panel">
        <h2 className="panel-title">New Alert Rule</h2>
        <div className="form-row">
          <label className="form-label">Name</label>
          <input className="form-select" value={name} onChange={e => setName(e.target.value)} placeholder="e.g. High CPU" />
        </div>
        <div className="form-row">
          <label className="form-label">Metric</label>
          <select className="form-select" value={metric} onChange={e => setMetric(e.target.value)}>
            {METRICS.map(m => <option key={m} value={m}>{m}</option>)}
          </select>
        </div>
        <div className="form-row">
          <label className="form-label">Condition</label>
          <div className="toggle-group">
            {([">", ">=", "<", "<="] as const).map(op => (
              <button key={op} className={`toggle-btn ${operator === op ? "active" : ""}`} onClick={() => setOperator(op)}>{op}</button>
            ))}
          </div>
          <input className="form-select" style={{ width: 100, marginLeft: 8 }} type="number" value={threshold} onChange={e => setThreshold(e.target.value)} />
        </div>
        <button className="send-btn" onClick={handleCreate} disabled={!name}>Create Rule</button>
      </section>

      {/* Rules list */}
      {rules.length > 0 && (
        <section className="panel">
          <h2 className="panel-title">Rules</h2>
          <div className="table-wrap">
            <table className="data-table">
              <thead><tr><th>Name</th><th>Metric</th><th>Condition</th><th>Status</th><th></th></tr></thead>
              <tbody>
                {rules.map(r => (
                  <tr key={r.id}>
                    <td style={{ fontWeight: 600 }}>{r.name}</td>
                    <td className="cell-mono">{r.metric_name}</td>
                    <td>{r.operator} {r.threshold}</td>
                    <td><span className={`status-pill ${r.enabled ? "status-online" : "status-offline"}`}>{r.enabled ? "enabled" : "disabled"}</span></td>
                    <td><button className="toggle-btn" style={{ color: "var(--red)" }} onClick={() => handleDelete(r.id)}>✕</button></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {/* Events */}
      <section className="panel">
        <h2 className="panel-title">Recent Firings</h2>
        {events.length === 0 ? (
          <div className="empty-state">No alerts have fired yet.</div>
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead><tr><th>Rule</th><th>Agent</th><th>Value</th><th>Fired</th><th>Resolved</th></tr></thead>
              <tbody>
                {events.map(e => (
                  <tr key={e.id}>
                    <td style={{ fontWeight: 600 }}>{e.rule_name}</td>
                    <td>{e.agent_id}</td>
                    <td>{e.metric_value.toFixed(2)}</td>
                    <td className="cell-dim">{new Date(e.fired_at).toLocaleString()}</td>
                    <td className="cell-dim">{e.resolved_at ? new Date(e.resolved_at).toLocaleString() : <span className="status-pill status-failed">ACTIVE</span>}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </main>
  );
}
