import { useEffect, useState, useCallback } from "react";
import { Agent, SessionSummary, getAgents, getSessions } from "../lib/api";

export function AgentsPage() {
  const [agents, setAgents] = useState<Agent[]>([]);
  const [sessions, setSessions] = useState<SessionSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [lastRefresh, setLastRefresh] = useState<Date>(new Date());

  const load = useCallback(async () => {
    try {
      const [a, s] = await Promise.all([getAgents(), getSessions()]);
      setAgents(a);
      setSessions(s);
      setError(null);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "failed to load");
    } finally {
      setLoading(false);
      setLastRefresh(new Date());
    }
  }, []);

  useEffect(() => {
    load();
    const t = setInterval(load, 15000);
    return () => clearInterval(t);
  }, [load]);

  const connectedIds = new Set(sessions.map((s) => s.agent_id));

  return (
    <main className="page">
      <header className="page-header">
        <h1 className="page-title">Agents</h1>
        <div className="page-meta">
          <span className="meta-badge">{agents.length} total</span>
          <span className="meta-badge online">{connectedIds.size} connected</span>
          <button className="refresh-btn" onClick={load}>↺ Refresh</button>
          <span className="last-refresh">
            {lastRefresh.toLocaleTimeString()}
          </span>
        </div>
      </header>

      {error && <div className="error-bar">{error}</div>}

      {loading && agents.length === 0 ? (
        <div className="empty-state">Loading agents…</div>
      ) : agents.length === 0 ? (
        <div className="empty-state">No agents registered yet.</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>Host</th>
                <th>OS</th>
                <th>Arch</th>
                <th>Tenant</th>
                <th>Last seen</th>
                <th>Connection</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              {agents.map((agent) => {
                const connected = connectedIds.has(agent.id);
                return (
                  <tr key={agent.id}>
                    <td className="cell-host">{agent.hostname}</td>
                    <td>{agent.os_family} {agent.os_version}</td>
                    <td>{agent.architecture}</td>
                    <td className="cell-dim">{agent.tenant_id}</td>
                    <td className="cell-dim">
                      {agent.last_seen_at
                        ? new Date(agent.last_seen_at).toLocaleTimeString()
                        : "—"}
                    </td>
                    <td>
                      <span className={`dot ${connected ? "dot-online" : "dot-offline"}`} />
                      {connected ? "WebSocket" : "Offline"}
                    </td>
                    <td>
                      <span className={`status-pill status-${agent.status}`}>
                        {agent.status}
                      </span>
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </main>
  );
}
