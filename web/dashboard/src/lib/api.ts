export type Agent = {
  id: string;
  tenant_id: string;
  hostname: string;
  os_family: string;
  os_version: string;
  architecture: string;
  status: string;
};

export type ComplianceFinding = {
  category: string;
  resource_id: string;
  status: string;
  reason?: string;
};

export type ComplianceReport = {
  agent_id: string;
  status: string;
  findings: ComplianceFinding[];
};

export type CommandRecord = {
  command_id: string;
  agent_id: string;
  command_type: string;
  status: string;
};

const API_BASE = "http://127.0.0.1:8080";
const COMPLIANCE_BASE = "http://127.0.0.1:8085";
const COMMAND_BASE = "http://127.0.0.1:8084";

export async function getAgents(): Promise<Agent[]> {
  const response = await fetch(`${API_BASE}/api/v1/agents`);
  if (!response.ok) {
    throw new Error("failed to load agents");
  }
  return response.json();
}

export async function getComplianceReports(): Promise<ComplianceReport[]> {
  const response = await fetch(`${COMPLIANCE_BASE}/api/v1/compliance/reports`);
  if (!response.ok) {
    throw new Error("failed to load compliance reports");
  }
  return response.json();
}

export async function getCommands(): Promise<CommandRecord[]> {
  const response = await fetch(`${COMMAND_BASE}/api/v1/commands`);
  if (!response.ok) {
    throw new Error("failed to load commands");
  }
  return response.json();
}
