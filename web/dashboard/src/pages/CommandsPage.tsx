import { useEffect, useState, useCallback } from "react";
import { CommandRecord, Agent, getCommands, getAgents, sendCommand } from "../lib/api";

export function CommandsPage() {
  const [commands, setCommands] = useState<CommandRecord[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const [success, setSuccess] = useState<string | null>(null);
  const [expanded, setExpanded] = useState<string | null>(null);

  const [selectedAgent, setSelectedAgent] = useState("");
  const [commandType, setCommandType] = useState<"ping" | "script">("ping");
  const [scriptBody, setScriptBody] = useState("");

  const load = useCallback(async () => {
    try {
      const [c, a] = await Promise.all([getCommands(), getAgents()]);
      setCommands(c);
      setAgents(a);
      if (!selectedAgent && a.length > 0) setSelectedAgent(a[0].id);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "failed to load");
    }
  }, [selectedAgent]);

  useEffect(() => {
    load();
    const t = setInterval(load, 10000);
    return () => clearInterval(t);
  }, [load]);

  async function handleSend() {
    if (!selectedAgent) return;
    setSending(true);
    setError(null);
    setSuccess(null);
    try {
      const resp = await sendCommand(
        selectedAgent,
        commandType,
        commandType === "script" ? scriptBody : undefined
      );
      setSuccess(`Dispatched ${resp.command_id}`);
      if (commandType === "script") setScriptBody("");
      await load();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "send failed");
    } finally {
      setSending(false);
    }
  }

  return (
    <main className="page">
      <header className="page-header">
        <h1 className="page-title">Commands</h1>
        <div className="page-meta">
          <span className="meta-badge">{commands.length} recorded</span>
        </div>
      </header>

      <section className="panel">
        <h2 className="panel-title">Dispatch Command</h2>
        <div className="form-row">
          <label className="form-label">Agent</label>
          <select
            className="form-select"
            value={selectedAgent}
            onChange={(e) => setSelectedAgent(e.target.value)}
          >
            {agents.map((a) => (
              <option key={a.id} value={a.id}>
                {a.hostname} ({a.id})
              </option>
            ))}
          </select>
        </div>
        <div className="form-row">
          <label className="form-label">Type</label>
          <div className="toggle-group">
            {(["ping", "script"] as const).map((t) => (
              <button
                key={t}
                className={`toggle-btn ${commandType === t ? "active" : ""}`}
                onClick={() => setCommandType(t)}
              >
                {t}
              </button>
            ))}
          </div>
        </div>
        {commandType === "script" && (
          <div className="form-row form-row-col">
            <label className="form-label">Script</label>
            <textarea
              className="form-textarea"
              rows={4}
              placeholder="echo hello && hostname"
              value={scriptBody}
              onChange={(e) => setScriptBody(e.target.value)}
            />
          </div>
        )}
        <button
          className="send-btn"
          onClick={handleSend}
          disabled={sending || !selectedAgent}
        >
          {sending ? "Sending…" : "Send →"}
        </button>
        {success && <div className="success-bar">{success}</div>}
        {error && <div className="error-bar">{error}</div>}
      </section>

      <section className="panel">
        <h2 className="panel-title">History</h2>
        {commands.length === 0 ? (
          <div className="empty-state">No commands yet.</div>
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>ID</th>
                  <th>Agent</th>
                  <th>Type</th>
                  <th>Status</th>
                  <th>Exit</th>
                  <th>Created</th>
                  <th>Completed</th>
                </tr>
              </thead>
              <tbody>
                {commands.map((cmd) => (
                  <>
                    <tr
                      key={cmd.command_id}
                      style={{ cursor: cmd.output ? "pointer" : "default" }}
                      onClick={() =>
                        cmd.output
                          ? setExpanded(expanded === cmd.command_id ? null : cmd.command_id)
                          : undefined
                      }
                    >
                      <td className="cell-dim cell-mono">{cmd.command_id.slice(-12)}</td>
                      <td>{cmd.agent_id}</td>
                      <td>{cmd.command_type}</td>
                      <td>
                        <span className={`status-pill status-${cmd.status}`}>
                          {cmd.status}
                        </span>
                      </td>
                      <td className="cell-dim">
                        {cmd.exit_code !== undefined ? cmd.exit_code : "—"}
                      </td>
                      <td className="cell-dim">
                        {cmd.created_at ? new Date(cmd.created_at).toLocaleTimeString() : "—"}
                      </td>
                      <td className="cell-dim">
                        {cmd.completed_at ? new Date(cmd.completed_at).toLocaleTimeString() : "—"}
                      </td>
                    </tr>
                    {expanded === cmd.command_id && cmd.output && (
                      <tr key={`${cmd.command_id}-output`}>
                        <td colSpan={7}>
                          <pre className="cmd-output">{cmd.output}</pre>
                          {cmd.error_message && (
                            <pre className="cmd-output" style={{ color: "var(--red)", marginTop: 4 }}>
                              {cmd.error_message}
                            </pre>
                          )}
                        </td>
                      </tr>
                    )}
                  </>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </main>
  );
}
