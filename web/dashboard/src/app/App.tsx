import { useState } from "react";
import { AgentsPage } from "../pages/AgentsPage";
import { CompliancePage } from "../pages/CompliancePage";
import { CommandsPage } from "../pages/CommandsPage";
import { MetricsPage } from "../pages/MetricsPage";
import { ScriptsPage } from "../pages/ScriptsPage";
import { AlertsPage } from "../pages/AlertsPage";

type PageKey = "agents" | "metrics" | "compliance" | "commands" | "scripts" | "alerts";

const NAV: { key: PageKey; label: string }[] = [
  { key: "agents",     label: "Agents" },
  { key: "metrics",    label: "Metrics" },
  { key: "compliance", label: "Compliance" },
  { key: "commands",   label: "Commands" },
  { key: "scripts",    label: "Scripts" },
  { key: "alerts",     label: "Alerts" },
];

export function App() {
  const [page, setPage] = useState<PageKey>("agents");
  return (
    <div className="shell">
      <aside className="sidebar">
        <div className="sidebar-brand">
          <span className="brand-mark">◈</span>
          <span className="brand-name">RMM</span>
        </div>
        <nav className="sidebar-nav">
          {NAV.map(({ key, label }) => (
            <button key={key} className={`nav-item ${page === key ? "active" : ""}`} onClick={() => setPage(key)}>
              {label}
            </button>
          ))}
        </nav>
        <div className="sidebar-footer"><span className="version-tag">v0.1.0</span></div>
      </aside>
      <section className="content">
        {page === "agents"     && <AgentsPage />}
        {page === "metrics"    && <MetricsPage />}
        {page === "compliance" && <CompliancePage />}
        {page === "commands"   && <CommandsPage />}
        {page === "scripts"    && <ScriptsPage />}
        {page === "alerts"     && <AlertsPage />}
      </section>
    </div>
  );
}
