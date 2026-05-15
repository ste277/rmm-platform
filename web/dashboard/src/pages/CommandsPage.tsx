import { useEffect, useState } from "react";
import { CommandRecord, getCommands } from "../lib/api";

export function CommandsPage() {
  const [commands, setCommands] = useState<CommandRecord[]>([]);

  useEffect(() => {
    getCommands().then(setCommands).catch(console.error);
  }, []);

  return (
    <main>
      <h1>Commands</h1>
      <div className="card-list">
        {commands.map((command) => (
          <article className="card" key={command.command_id}>
            <h2>{command.command_type}</h2>
            <p>Command ID: {command.command_id}</p>
            <p>Agent: {command.agent_id}</p>
            <p>Status: {command.status}</p>
          </article>
        ))}
      </div>
    </main>
  );
}
