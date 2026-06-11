package api

type HealthResponse struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type AgentSummary struct {
	ID           string `json:"id"`
	TenantID     string `json:"tenant_id"`
	Hostname     string `json:"hostname"`
	OSFamily     string `json:"os_family"`
	OSVersion    string `json:"os_version"`
	Architecture string `json:"architecture"`
	Status       string `json:"status"`
	LastSeenAt   string `json:"last_seen_at,omitempty"`
}

type IngestRequest struct {
	Type         string              `json:"type"`
	AgentID      string              `json:"agent_id,omitempty"`
	TenantID     string              `json:"tenant_id,omitempty"`
	Hostname     string              `json:"hostname,omitempty"`
	OSFamily     string              `json:"os_family,omitempty"`
	OSVersion    string              `json:"os_version,omitempty"`
	Architecture string              `json:"architecture,omitempty"`
	Software     []map[string]string `json:"software,omitempty"`
	Status       string              `json:"status,omitempty"`
	Findings     []ComplianceFinding `json:"findings,omitempty"`
	Points       []MetricPoint       `json:"points,omitempty"`
	Payload      map[string]any      `json:"payload,omitempty"`
}

type MetricPoint struct {
	Name            string            `json:"name"`
	Value           float64           `json:"value"`
	CollectedAtUnix int64             `json:"collected_at_unix"`
	Tags            map[string]string `json:"tags,omitempty"`
}

type AcceptedResponse struct {
	Status string `json:"status"`
}

// Registration

type RegistrationRequest struct {
	TenantID        string `json:"tenant_id"`
	EnrollmentToken string `json:"enrollment_token"`
	Hostname        string `json:"hostname"`
	OSFamily        string `json:"os_family"`
	OSVersion       string `json:"os_version"`
	Architecture    string `json:"architecture"`
}

type RegistrationResponse struct {
	AgentID   string `json:"agent_id"`
	BrokerURL string `json:"broker_url"`
}

// Inventory

type InventoryRecord struct {
	AgentID      string              `json:"agent_id"`
	Hostname     string              `json:"hostname"`
	OSFamily     string              `json:"os_family"`
	OSVersion    string              `json:"os_version"`
	Architecture string              `json:"architecture"`
	Software     []map[string]string `json:"software,omitempty"`
	Attributes   map[string]any      `json:"attributes,omitempty"`
}

type InventoryIngestResponse struct {
	Status string `json:"status"`
}

// Commands

type CommandCreateRequest struct {
	AgentID          string   `json:"agent_id"`
	CommandType      string   `json:"command_type"`
	ScriptBody       string   `json:"script_body,omitempty"`
	Args             []string `json:"args,omitempty"`
	TimeoutSec       int      `json:"timeout_seconds,omitempty"`
	RequiresApproval bool     `json:"requires_approval,omitempty"`
}

type CommandResponse struct {
	CommandID string `json:"command_id"`
	Status    string `json:"status"`
}

type CommandRecord struct {
	CommandID    string `json:"command_id"`
	AgentID      string `json:"agent_id"`
	CommandType  string `json:"command_type"`
	Status       string `json:"status"`
	ExitCode     *int   `json:"exit_code,omitempty"`
	Output       string `json:"output,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`
	CompletedAt  string `json:"completed_at,omitempty"`
}

// Command result sent back from agent over WebSocket
type CommandResult struct {
	Type      string `json:"type"` // "command_result"
	CommandID string `json:"command_id"`
	AgentID   string `json:"agent_id"`
	ExitCode  int    `json:"exit_code"`
	Output    string `json:"output"`
	Error     string `json:"error,omitempty"`
	RanAt     int64  `json:"ran_at_unix"`
}

// Compliance

type ComplianceFinding struct {
	Category   string `json:"category"`
	ResourceID string `json:"resource_id"`
	Status     string `json:"status"`
	Reason     string `json:"reason,omitempty"`
	ActionHint string `json:"action_hint,omitempty"`
}

type ComplianceReport struct {
	AgentID   string              `json:"agent_id"`
	Status    string              `json:"status"`
	Findings  []ComplianceFinding `json:"findings,omitempty"`
	CreatedAt string              `json:"created_at,omitempty"`
}

// Sessions

type SessionSummary struct {
	AgentID     string `json:"agent_id"`
	RemoteAddr  string `json:"remote_addr"`
	ConnectedAt string `json:"connected_at"`
}

// Metrics

type MetricSample struct {
	MetricName  string            `json:"metric_name"`
	MetricValue float64           `json:"metric_value"`
	CollectedAt string            `json:"collected_at"`
	Tags        map[string]string `json:"tags,omitempty"`
}

type AgentMetrics struct {
	AgentID string         `json:"agent_id"`
	Points  []MetricSample `json:"points"`
}

// Authentication

type APIKey struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	Name      string `json:"name"`
	KeyPrefix string `json:"key_prefix"` // first 8 chars shown in UI
	CreatedAt string `json:"created_at"`
}

type CreateAPIKeyRequest struct {
	TenantID string `json:"tenant_id"`
	Name     string `json:"name"`
}

type CreateAPIKeyResponse struct {
	APIKey
	RawKey string `json:"raw_key"` // shown once only
}

type EnrollmentToken struct {
	ID        string `json:"id"`
	TenantID  string `json:"tenant_id"`
	Label     string `json:"label"`
	Used      bool   `json:"used"`
	ExpiresAt string `json:"expires_at,omitempty"`
	CreatedAt string `json:"created_at"`
}

type CreateEnrollmentTokenRequest struct {
	TenantID  string `json:"tenant_id"`
	Label     string `json:"label"`
	ExpiresAt string `json:"expires_at,omitempty"`
}
