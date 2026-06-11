import { useEffect, useState, useCallback } from "react";
import { Agent, MetricSample, getAgents, getMetrics } from "../lib/api";

const METRIC_LABELS: Record<string, string> = {
  "system.cpu.percent":       "CPU %",
  "system.mem.percent":       "Memory %",
  "system.mem.used_bytes":    "Memory Used",
  "system.disk.percent":      "Disk %",
  "system.disk.used_bytes":   "Disk Used",
};

function formatValue(name: string, value: number): string {
  if (name.endsWith("_bytes")) {
    const gb = value / 1024 / 1024 / 1024;
    return gb >= 1 ? `${gb.toFixed(1)} GB` : `${(value / 1024 / 1024).toFixed(0)} MB`;
  }
  if (name.endsWith("percent")) return `${value.toFixed(1)}%`;
  return value.toFixed(2);
}

function Sparkline({ points, max }: { points: number[]; max: number }) {
  if (points.length < 2) return <span className="cell-dim">—</span>;
  const w = 120, h = 32, pad = 2;
  const step = (w - pad * 2) / (points.length - 1);
  const coords = points.map((v, i) => {
    const x = pad + i * step;
    const y = pad + (h - pad * 2) * (1 - v / (max || 1));
    return `${x},${y}`;
  });
  const last = points[points.length - 1];
  const pct = max ? (last / max) * 100 : 0;
  const colour = pct > 85 ? "#ef4444" : pct > 60 ? "#eab308" : "#22c55e";
  return (
    <svg width={w} height={h} style={{ overflow: "visible" }}>
      <polyline
        points={coords.join(" ")}
        fill="none"
        stroke={colour}
        strokeWidth="1.5"
        strokeLinejoin="round"
      />
    </svg>
  );
}

type MetricGroup = { latest: number; history: number[]; max: number };

function groupMetrics(points: MetricSample[]): Record<string, MetricGroup> {
  const groups: Record<string, { values: { v: number; t: string }[] }> = {};
  for (const p of points) {
    if (!groups[p.metric_name]) groups[p.metric_name] = { values: [] };
    groups[p.metric_name].values.push({ v: p.metric_value, t: p.collected_at });
  }
  const result: Record<string, MetricGroup> = {};
  for (const [name, g] of Object.entries(groups)) {
    const sorted = g.values.sort((a, b) => a.t.localeCompare(b.t));
    const history = sorted.map((x) => x.v);
    const max = name.endsWith("percent") ? 100 : Math.max(...history) * 1.1;
    result[name] = { latest: history[history.length - 1], history, max };
  }
  return result;
}

export function MetricsPage() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [selectedAgent, setSelectedAgent] = useState("");
  const [groups, setGroups] = useState<Record<string, MetricGroup>>({});
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);
  const [lastRefresh, setLastRefresh] = useState<Date>(new Date());

  useEffect(() => {
    getAgents()
      .then((a) => {
        setAgents(a);
        if (a.length > 0) setSelectedAgent(a[0].id);
      })
      .catch(console.error);
  }, []);

  const load = useCallback(async () => {
    if (!selectedAgent) return;
    setLoading(true);
    setError(null);
    try {
      const data = await getMetrics(selectedAgent);
      setGroups(groupMetrics(data.points ?? []));
      setLastRefresh(new Date());
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "failed to load metrics");
    } finally {
      setLoading(false);
    }
  }, [selectedAgent]);

  useEffect(() => {
    load();
    const t = setInterval(load, 60000);
    return () => clearInterval(t);
  }, [load]);

  const displayMetrics = Object.keys(METRIC_LABELS).filter((k) => groups[k]);

  return (
    <main className="page">
      <header className="page-header">
        <h1 className="page-title">Metrics</h1>
        <div className="page-meta">
          <select
            className="form-select"
            style={{ width: "auto", minWidth: 200 }}
            value={selectedAgent}
            onChange={(e) => setSelectedAgent(e.target.value)}
          >
            {agents.map((a) => (
              <option key={a.id} value={a.id}>
                {a.hostname}
              </option>
            ))}
          </select>
          <button className="refresh-btn" onClick={load}>↺ Refresh</button>
          <span className="last-refresh">{lastRefresh.toLocaleTimeString()}</span>
        </div>
      </header>

      {error && <div className="error-bar">{error}</div>}

      {loading && displayMetrics.length === 0 ? (
        <div className="empty-state">Loading metrics…</div>
      ) : displayMetrics.length === 0 ? (
        <div className="empty-state">
          No metrics yet. The agent sends metrics every 60 seconds.
        </div>
      ) : (
        <>
          {/* Summary cards */}
          <div className="metric-cards">
            {displayMetrics.map((name) => {
              const g = groups[name];
              const pct = name.endsWith("percent") ? g.latest : null;
              const level =
                pct !== null
                  ? pct > 85
                    ? "danger"
                    : pct > 60
                    ? "warn"
                    : "ok"
                  : "ok";
              return (
                <div key={name} className={`metric-card metric-card-${level}`}>
                  <div className="metric-card-label">{METRIC_LABELS[name]}</div>
                  <div className="metric-card-value">
                    {formatValue(name, g.latest)}
                  </div>
                  <Sparkline points={g.history.slice(-30)} max={g.max} />
                </div>
              );
            })}
          </div>

          {/* Detail table */}
          <section className="panel">
            <h2 className="panel-title">Latest Samples</h2>
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Metric</th>
                    <th>Latest</th>
                    <th>Trend (last 30)</th>
                    <th>Samples</th>
                  </tr>
                </thead>
                <tbody>
                  {displayMetrics.map((name) => {
                    const g = groups[name];
                    return (
                      <tr key={name}>
                        <td className="cell-mono">{name}</td>
                        <td style={{ fontWeight: 600 }}>
                          {formatValue(name, g.latest)}
                        </td>
                        <td>
                          <Sparkline points={g.history.slice(-30)} max={g.max} />
                        </td>
                        <td className="cell-dim">{g.history.length}</td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </section>
        </>
      )}
    </main>
  );
}
