import { useEffect, useState, useCallback } from "react";

type Script = {
  id: string;
  name: string;
  description: string;
  body: string;
  created_at?: string;
};

type Agent = { id: string; hostname: string };

const GATEWAY = "http://127.0.0.1:8080";
const BROKER  = "http://127.0.0.1:8081";

export function ScriptsPage() {
  const [scripts, setScripts] = useState<Script[]>([]);
  const [agents, setAgents] = useState<Agent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  // New script form
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [body, setBody] = useState("");

  // Run form
  const [runScript, setRunScript] = useState<Script | null>(null);
  const [runAgent, setRunAgent] = useState("");
  const [running, setRunning] = useState(false);

  const load = useCallback(async () => {
    try {
      const [sc, ag] = await Promise.all([
        fetch(`${GATEWAY}/api/v1/scripts?tenant_id=dev-tenant-001`).then(r => r.json()),
        fetch(`${GATEWAY}/api/v1/agents`).then(r => r.json()),
      ]);
      setScripts(sc ?? []);
      setAgents(ag ?? []);
      if (!runAgent && ag?.length > 0) setRunAgent(ag[0].id);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "failed to load");
    }
  }, [runAgent]);

  useEffect(() => { load(); }, [load]);

  async function handleSave() {
    if (!name || !body) return;
    setError(null);
    try {
      const r = await fetch(`${GATEWAY}/api/v1/scripts`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ tenant_id: "dev-tenant-001", name, description, body }),
      });
      if (!r.ok) throw new Error(await r.text());
      setName(""); setDescription(""); setBody("");
      setSuccess("Script saved");
      await load();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "save failed");
    }
  }

  async function handleDelete(id: string) {
    try {
      await fetch(`${GATEWAY}/api/v1/scripts/${id}`, { method: "DELETE" });
      await load();
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "delete failed");
    }
  }

  async function handleRun() {
    if (!runScript || !runAgent) return;
    setRunning(true);
    setError(null);
    setSuccess(null);
    try {
      const r = await fetch(`${BROKER}/api/v1/agent-commands/${runAgent}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ command_type: "script", script_body: runScript.body }),
      });
      if (!r.ok) throw new Error(await r.text());
      setSuccess(`Script "${runScript.name}" dispatched to ${runAgent}`);
      setRunScript(null);
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : "run failed");
    } finally {
      setRunning(false);
    }
  }

  return (
    <main className="page">
      <header className="page-header">
        <h1 className="page-title">Script Library</h1>
        <div className="page-meta">
          <span className="meta-badge">{scripts.length} scripts</span>
        </div>
      </header>

      {error && <div className="error-bar">{error}</div>}
      {success && <div className="success-bar">{success}</div>}

      {/* New script */}
      <section className="panel">
        <h2 className="panel-title">New Script</h2>
        <div className="form-row">
          <label className="form-label">Name</label>
          <input className="form-select" value={name} onChange={e => setName(e.target.value)} placeholder="e.g. Check disk usage" />
        </div>
        <div className="form-row">
          <label className="form-label">Description</label>
          <input className="form-select" value={description} onChange={e => setDescription(e.target.value)} placeholder="Optional" />
        </div>
        <div className="form-row form-row-col">
          <label className="form-label">Body</label>
          <textarea className="form-textarea" rows={5} value={body} onChange={e => setBody(e.target.value)} placeholder="#!/bin/bash&#10;df -h" />
        </div>
        <button className="send-btn" onClick={handleSave} disabled={!name || !body}>Save Script</button>
      </section>

      {/* Script list */}
      {scripts.length === 0 ? (
        <div className="empty-state">No scripts yet. Save one above.</div>
      ) : (
        <section className="panel">
          <h2 className="panel-title">Saved Scripts</h2>
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr><th>Name</th><th>Description</th><th>Created</th><th>Actions</th></tr>
              </thead>
              <tbody>
                {scripts.map(sc => (
                  <tr key={sc.id}>
                    <td style={{ fontWeight: 600 }}>{sc.name}</td>
                    <td className="cell-dim">{sc.description || "—"}</td>
                    <td className="cell-dim">{sc.created_at ? new Date(sc.created_at).toLocaleDateString() : "—"}</td>
                    <td>
                      <div style={{ display: "flex", gap: 8 }}>
                        <button className="toggle-btn active" onClick={() => { setRunScript(sc); setSuccess(null); setError(null); }}>▶ Run</button>
                        <button className="toggle-btn" style={{ color: "var(--red)" }} onClick={() => handleDelete(sc.id)}>✕ Delete</button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </section>
      )}

      {/* Run modal */}
      {runScript && (
        <div className="modal-backdrop" onClick={() => setRunScript(null)}>
          <div className="modal" onClick={e => e.stopPropagation()}>
            <h2 className="panel-title">Run: {runScript.name}</h2>
            <pre className="cmd-output" style={{ marginBottom: 16 }}>{runScript.body}</pre>
            <div className="form-row">
              <label className="form-label">Agent</label>
              <select className="form-select" value={runAgent} onChange={e => setRunAgent(e.target.value)}>
                {agents.map(a => <option key={a.id} value={a.id}>{a.hostname} ({a.id})</option>)}
              </select>
            </div>
            <div style={{ display: "flex", gap: 8, marginTop: 8 }}>
              <button className="send-btn" onClick={handleRun} disabled={running}>{running ? "Running…" : "Dispatch →"}</button>
              <button className="refresh-btn" onClick={() => setRunScript(null)}>Cancel</button>
            </div>
          </div>
        </div>
      )}
    </main>
  );
}
