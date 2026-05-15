import { useEffect, useState } from "react";
import { Agent, getAgents } from "../lib/api";

export function AgentsPage() {
  const [agents, setAgents] = useState<Agent[]>([]);

  useEffect(() => {
    getAgents().then(setAgents).catch(console.error);
  }, []);

  return (
    <main>
      <h1>Agents</h1>
      <div className="card-list">
        {agents.map((agent) => (
          <article className="card" key={agent.id}>
            <h2>{agent.hostname}</h2>
            <p>{agent.os_family} {agent.os_version}</p>
            <p>{agent.architecture}</p>
            <p>Status: {agent.status}</p>
          </article>
        ))}
      </div>
    </main>
  );
}
