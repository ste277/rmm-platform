export type Agent = {
  id: string;
  tenant_id: string;
  hostname: string;
  os_family: string;
  os_version: string;
  architecture: string;
  status: string;
  last_seen_at?: string;
};

export type ComplianceFinding = {
  category: string;
  resource_id: string;
  status: string;
  reason?: string;
  action_hint?: string;
};

export type ComplianceReport = {
  agent_id: string;
  status: string;
  findings: ComplianceFinding[];
  created_at?: string;
};

export type CommandRecord = {
  command_id: string;
  agent_id: string;
  command_type: string;
  status: string;
  exit_code?: number;
  output?: string;
  error_message?: string;
  created_at?: string;
  completed_at?: string;
};

export type SessionSummary = {
  agent_id: string;
  remote_addr: string;
  connected_at: string;
};

export type MetricSample = {
  metric_name: string;
  metric_value: number;
  collected_at: string;
  tags?: Record<string, string>;
};

export type AgentMetrics = {
  agent_id: string;
  points: MetricSample[] | null;
};

const GATEWAY    = "http://127.0.0.1:8080";
const BROKER     = "http://127.0.0.1:8081";
const COMPLIANCE = "http://127.0.0.1:8085";
const COMMAND    = "http://127.0.0.1:8084";

async function get<T>(url: string): Promise<T> {
  const r = await fetch(url);
  if (!r.ok) throw new Error(`${r.status} ${r.statusText}`);
  return r.json();
}

export const getAgents            = () => get<Agent[]>(`${GATEWAY}/api/v1/agents`);
export const getSessions          = () => get<SessionSummary[]>(`${BROKER}/api/v1/sessions`);
export const getComplianceReports = () => get<ComplianceReport[]>(`${COMPLIANCE}/api/v1/compliance/reports`);
export const getCommands          = () => get<CommandRecord[]>(`${COMMAND}/api/v1/commands`);
export const getMetrics           = (agentId: string) =>
  get<AgentMetrics>(`${GATEWAY}/api/v1/metrics/${agentId}`);

export async function sendCommand(
  agentId: string,
  commandType: string,
  scriptBody?: string
): Promise<{ command_id: string; status: string }> {
  const r = await fetch(`${BROKER}/api/v1/agent-commands/${agentId}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ command_type: commandType, script_body: scriptBody ?? "" }),
  });
  if (!r.ok) throw new Error(`${r.status} ${r.statusText}`);
  return r.json();
}
