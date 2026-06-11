import { useEffect, useState, useCallback } from "react";
import { ComplianceReport, getComplianceReports } from "../lib/api";

const STATUS_COLOURS: Record<string, string> = {
  installed: "green",
  compliant: "green",
  missing: "red",
  failed: "red",
  non_compliant: "red",
  needs_review: "yellow",
  blocked_by_policy: "orange",
  blocked_by_prerequisite: "orange",
  exempted: "blue",
  pending_reboot: "yellow",
  source_unreachable: "red",
};

export function CompliancePage() {
  const [reports, setReports] = useState<ComplianceReport[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [lastRefresh, setLastRefresh] = useState<Date>(new Date());

  const load = useCallback(async () => {
    try {
      const r = await getComplianceReports();
      setReports(r);
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
    const t = setInterval(load, 30000);
    return () => clearInterval(t);
  }, [load]);

  const compliant = reports.filter((r) => r.status === "compliant").length;
  const nonCompliant = reports.filter((r) => r.status === "non_compliant").length;

  return (
    <main className="page">
      <header className="page-header">
        <h1 className="page-title">Compliance</h1>
        <div className="page-meta">
          <span className="meta-badge online">{compliant} compliant</span>
          {nonCompliant > 0 && (
            <span className="meta-badge danger">{nonCompliant} non-compliant</span>
          )}
          <button className="refresh-btn" onClick={load}>↺ Refresh</button>
          <span className="last-refresh">{lastRefresh.toLocaleTimeString()}</span>
        </div>
      </header>

      {error && <div className="error-bar">{error}</div>}

      {loading && reports.length === 0 ? (
        <div className="empty-state">Loading reports…</div>
      ) : reports.length === 0 ? (
        <div className="empty-state">No compliance reports yet. The agent sends one every 5 minutes.</div>
      ) : (
        <div className="report-list">
          {reports.map((report) => (
            <section className="panel" key={report.agent_id + (report.created_at ?? "")}>
              <div className="report-header">
                <h2 className="panel-title">{report.agent_id}</h2>
                <span className={`status-pill status-${report.status}`}>
                  {report.status}
                </span>
                {report.created_at && (
                  <span className="cell-dim">
                    {new Date(report.created_at).toLocaleString()}
                  </span>
                )}
              </div>

              {report.findings && report.findings.length > 0 ? (
                <div className="table-wrap">
                  <table className="data-table">
                    <thead>
                      <tr>
                        <th>Category</th>
                        <th>Resource</th>
                        <th>Status</th>
                        <th>Reason</th>
                        <th>Action</th>
                      </tr>
                    </thead>
                    <tbody>
                      {report.findings.map((f) => (
                        <tr key={`${f.category}-${f.resource_id}`}>
                          <td>{f.category}</td>
                          <td className="cell-mono">{f.resource_id}</td>
                          <td>
                            <span
                              className={`status-pill status-colour-${STATUS_COLOURS[f.status] ?? "grey"}`}
                            >
                              {f.status}
                            </span>
                          </td>
                          <td className="cell-dim">{f.reason ?? "—"}</td>
                          <td className="cell-dim">{f.action_hint ?? "—"}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <p className="cell-dim">No findings.</p>
              )}
            </section>
          ))}
        </div>
      )}
    </main>
  );
}
