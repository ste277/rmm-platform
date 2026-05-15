import { useState } from "react";
import { AgentsPage } from "../pages/AgentsPage";
import { CompliancePage } from "../pages/CompliancePage";
import { CommandsPage } from "../pages/CommandsPage";

type PageKey = "agents" | "compliance" | "commands";

export function App() {
  const [page, setPage] = useState<PageKey>("agents");

  return (
    <div className="shell">
      <aside className="sidebar">
        <h1>RMM Console</h1>
        <button onClick={() => setPage("agents")}>Agents</button>
        <button onClick={() => setPage("compliance")}>Compliance</button>
        <button onClick={() => setPage("commands")}>Commands</button>
      </aside>
      <section className="content">
        {page === "agents" && <AgentsPage />}
        {page === "compliance" && <CompliancePage />}
        {page === "commands" && <CommandsPage />}
      </section>
    </div>
  );
}
