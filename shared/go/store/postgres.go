package store

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"example.com/rmm-shared/api"
)

type Store struct {
	db *sql.DB
}

func Open(databaseURL string) (*Store, error) {
	if databaseURL == "" {
		return nil, nil
	}

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

// ---------------------------------------------------------------------------
// Ingest dispatcher
// ---------------------------------------------------------------------------

func (s *Store) ProcessIngest(ctx context.Context, req api.IngestRequest) error {
	if s == nil {
		return nil
	}

	switch req.Type {
	case "heartbeat":
		return s.RecordHeartbeat(ctx, req)
	case "inventory":
		if req.AgentID == "" {
			return errors.New("agent_id is required for inventory")
		}
		return s.UpsertInventory(ctx, api.InventoryRecord{
			AgentID:      req.AgentID,
			Hostname:     req.Hostname,
			OSFamily:     req.OSFamily,
			OSVersion:    req.OSVersion,
			Architecture: req.Architecture,
			Software:     req.Software,
			Attributes:   req.Payload,
		})
	case "metrics":
		if req.AgentID == "" {
			return errors.New("agent_id is required for metrics")
		}
		return s.SaveMetricPoints(ctx, req.AgentID, req.Points)
	case "compliance":
		if req.AgentID == "" {
			return errors.New("agent_id is required for compliance")
		}
		return s.SaveComplianceReport(ctx, api.ComplianceReport{
			AgentID:  req.AgentID,
			Status:   req.Status,
			Findings: req.Findings,
		})
	case "command_result":
		return s.RecordCommandResult(ctx, req)
	default:
		return nil
	}
}

// ---------------------------------------------------------------------------
// Agents
// ---------------------------------------------------------------------------

func (s *Store) RecordHeartbeat(ctx context.Context, req api.IngestRequest) error {
	if req.AgentID == "" || req.TenantID == "" {
		return errors.New("agent_id and tenant_id are required for heartbeat")
	}

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tenants (id, name)
		VALUES ($1, $2)
		ON CONFLICT (id) DO NOTHING
	`, req.TenantID, req.TenantID)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agents (id, tenant_id, hostname, os_family, os_version, architecture, status, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'online', NOW())
		ON CONFLICT (id) DO UPDATE SET
			tenant_id    = EXCLUDED.tenant_id,
			hostname     = EXCLUDED.hostname,
			os_family    = EXCLUDED.os_family,
			os_version   = EXCLUDED.os_version,
			architecture = EXCLUDED.architecture,
			status       = 'online',
			last_seen_at = NOW()
	`, req.AgentID, req.TenantID, req.Hostname, req.OSFamily, req.OSVersion, req.Architecture)
	return err
}

// MarkAgentsOffline sets agents whose last heartbeat is older than the given
// threshold to 'offline'. Call this periodically from a background goroutine.
func (s *Store) MarkAgentsOffline(ctx context.Context, threshold time.Duration) error {
	if s == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE agents
		SET status = 'offline'
		WHERE status = 'online'
		  AND last_seen_at < NOW() - $1::interval
	`, threshold.String())
	return err
}

func (s *Store) ListAgents(ctx context.Context) ([]api.AgentSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, hostname, os_family, os_version, architecture, status,
		       COALESCE(last_seen_at::text, '')
		FROM agents
		ORDER BY hostname
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []api.AgentSummary
	for rows.Next() {
		var a api.AgentSummary
		if err := rows.Scan(&a.ID, &a.TenantID, &a.Hostname, &a.OSFamily,
			&a.OSVersion, &a.Architecture, &a.Status, &a.LastSeenAt); err != nil {
			return nil, err
		}
		agents = append(agents, a)
	}
	return agents, rows.Err()
}

func (s *Store) RegisterAgent(ctx context.Context, req api.RegistrationRequest) (api.RegistrationResponse, error) {
	// Validate enrollment token if provided
	if req.EnrollmentToken != "" {
		if err := s.consumeEnrollmentToken(ctx, req.TenantID, req.EnrollmentToken); err != nil {
			return api.RegistrationResponse{}, fmt.Errorf("invalid enrollment token: %w", err)
		}
	}

	agentID := fmt.Sprintf("%s-%s", req.TenantID, req.Hostname)

	_, err := s.db.ExecContext(ctx, `
		INSERT INTO tenants (id, name)
		VALUES ($1, $2)
		ON CONFLICT (id) DO NOTHING
	`, req.TenantID, req.TenantID)
	if err != nil {
		return api.RegistrationResponse{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agents (id, tenant_id, hostname, os_family, os_version, architecture, status, last_seen_at)
		VALUES ($1, $2, $3, $4, $5, $6, 'online', NOW())
		ON CONFLICT (id) DO UPDATE SET
			os_family    = EXCLUDED.os_family,
			os_version   = EXCLUDED.os_version,
			architecture = EXCLUDED.architecture,
			status       = 'online',
			last_seen_at = NOW()
	`, agentID, req.TenantID, req.Hostname, req.OSFamily, req.OSVersion, req.Architecture)
	if err != nil {
		return api.RegistrationResponse{}, err
	}

	return api.RegistrationResponse{
		AgentID:   agentID,
		BrokerURL: "ws://127.0.0.1:8081/ws?agent_id=" + agentID,
	}, nil
}

// ---------------------------------------------------------------------------
// Inventory
// ---------------------------------------------------------------------------

func (s *Store) UpsertInventory(ctx context.Context, rec api.InventoryRecord) error {
	payload, err := json.Marshal(rec.Attributes)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE agents
		SET hostname = $2, os_family = $3, os_version = $4, architecture = $5, last_seen_at = NOW()
		WHERE id = $1
	`, rec.AgentID, rec.Hostname, rec.OSFamily, rec.OSVersion, rec.Architecture)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agent_inventory_snapshots (agent_id, payload)
		VALUES ($1, $2::jsonb)
	`, rec.AgentID, string(payload))
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `DELETE FROM installed_software WHERE agent_id = $1`, rec.AgentID)
	if err != nil {
		return err
	}

	for _, sw := range rec.Software {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO installed_software (agent_id, name, version, publisher, source)
			VALUES ($1, $2, $3, $4, $5)
		`, rec.AgentID, sw["name"], sw["version"], sw["publisher"], sw["source"])
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListInventory(ctx context.Context) ([]api.InventoryRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, hostname, os_family, os_version, architecture
		FROM agents ORDER BY hostname
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []api.InventoryRecord
	for rows.Next() {
		var rec api.InventoryRecord
		if err := rows.Scan(&rec.AgentID, &rec.Hostname, &rec.OSFamily, &rec.OSVersion, &rec.Architecture); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// ---------------------------------------------------------------------------
// Commands
// ---------------------------------------------------------------------------

func (s *Store) CreateCommand(ctx context.Context, req api.CommandCreateRequest) (api.CommandResponse, error) {
	commandID := fmt.Sprintf("cmd-%d", time.Now().UnixNano())
	payload, err := json.Marshal(req)
	if err != nil {
		return api.CommandResponse{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO commands (id, tenant_id, agent_id, command_type, payload, status, requested_by)
		VALUES ($1, 'dev-tenant-001', $2, $3, $4::jsonb, 'queued', 'dashboard')
	`, commandID, req.AgentID, req.CommandType, string(payload))
	if err != nil {
		return api.CommandResponse{}, err
	}
	return api.CommandResponse{CommandID: commandID, Status: "queued"}, nil
}

// RecordCommandResult updates a command record with the result sent back by the agent.
func (s *Store) RecordCommandResult(ctx context.Context, req api.IngestRequest) error {
	if s == nil || req.Payload == nil {
		return nil
	}

	commandID, _ := req.Payload["command_id"].(string)
	exitCode, _ := req.Payload["exit_code"].(float64)
	output, _ := req.Payload["output"].(string)
	errMsg, _ := req.Payload["error"].(string)

	if commandID == "" {
		return errors.New("command_id missing in command_result payload")
	}

	status := "completed"
	if int(exitCode) != 0 {
		status = "failed"
	}

	_, err := s.db.ExecContext(ctx, `
		UPDATE commands
		SET status        = $2,
		    exit_code     = $3,
		    output        = $4,
		    error_message = $5,
		    finished_at   = NOW()
		WHERE id = $1
	`, commandID, status, int(exitCode), output, errMsg)
	return err
}

func (s *Store) ListCommands(ctx context.Context) ([]api.CommandRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, agent_id, command_type, status,
		       COALESCE(exit_code::text, ''),
		       COALESCE(output, ''),
		       COALESCE(error_message, ''),
		       created_at::text,
		       COALESCE(finished_at::text, '')
		FROM commands
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []api.CommandRecord
	for rows.Next() {
		var rec api.CommandRecord
		var exitCodeStr string
		if err := rows.Scan(&rec.CommandID, &rec.AgentID, &rec.CommandType, &rec.Status,
			&exitCodeStr, &rec.Output, &rec.ErrorMessage,
			&rec.CreatedAt, &rec.CompletedAt); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// ---------------------------------------------------------------------------
// Metrics
// ---------------------------------------------------------------------------

func (s *Store) SaveMetricPoints(ctx context.Context, agentID string, points []api.MetricPoint) error {
	for _, point := range points {
		collectedAt := time.Unix(point.CollectedAtUnix, 0)
		if point.CollectedAtUnix == 0 {
			collectedAt = time.Now()
		}
		tagsJSON, err := json.Marshal(point.Tags)
		if err != nil {
			return err
		}
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO metric_samples (agent_id, metric_name, metric_value, collected_at, tags)
			VALUES ($1, $2, $3, $4, $5::jsonb)
		`, agentID, point.Name, point.Value, collectedAt, string(tagsJSON))
		if err != nil {
			return err
		}
	}
	return nil
}

// GetRecentMetrics returns the last N metric samples per metric name for an agent.
func (s *Store) GetRecentMetrics(ctx context.Context, agentID string, limit int) ([]api.MetricSample, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT metric_name, metric_value, collected_at::text, tags
		FROM metric_samples
		WHERE agent_id = $1
		ORDER BY collected_at DESC
		LIMIT $2
	`, agentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var samples []api.MetricSample
	for rows.Next() {
		var s api.MetricSample
		var tagsJSON string
		if err := rows.Scan(&s.MetricName, &s.MetricValue, &s.CollectedAt, &tagsJSON); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(tagsJSON), &s.Tags)
		samples = append(samples, s)
	}
	return samples, rows.Err()
}

// ---------------------------------------------------------------------------
// Compliance
// ---------------------------------------------------------------------------

func (s *Store) SaveComplianceReport(ctx context.Context, req api.ComplianceReport) error {
	var reportID int64
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO compliance_reports (agent_id, report_type, status)
		VALUES ($1, 'software', $2)
		RETURNING id
	`, req.AgentID, req.Status).Scan(&reportID)
	if err != nil {
		return err
	}
	for _, finding := range req.Findings {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO compliance_findings (report_id, category, resource_id, status, reason, action_hint)
			VALUES ($1, $2, $3, $4, $5, $6)
		`, reportID, finding.Category, finding.ResourceID, finding.Status, finding.Reason, finding.ActionHint)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListComplianceReports(ctx context.Context) ([]api.ComplianceReport, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT ON (agent_id) id, agent_id, status, generated_at::text
		FROM compliance_reports
		ORDER BY agent_id, generated_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var reports []api.ComplianceReport
	for rows.Next() {
		var reportID int64
		var report api.ComplianceReport
		if err := rows.Scan(&reportID, &report.AgentID, &report.Status, &report.CreatedAt); err != nil {
			return nil, err
		}
		findings, err := s.listFindings(ctx, reportID)
		if err != nil {
			return nil, err
		}
		report.Findings = findings
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

func (s *Store) listFindings(ctx context.Context, reportID int64) ([]api.ComplianceFinding, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT category, resource_id, status, COALESCE(reason, ''), COALESCE(action_hint, '')
		FROM compliance_findings
		WHERE report_id = $1
		ORDER BY id
	`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var findings []api.ComplianceFinding
	for rows.Next() {
		var f api.ComplianceFinding
		if err := rows.Scan(&f.Category, &f.ResourceID, &f.Status, &f.Reason, &f.ActionHint); err != nil {
			return nil, err
		}
		findings = append(findings, f)
	}
	return findings, rows.Err()
}

// ---------------------------------------------------------------------------
// Authentication — API Keys
// ---------------------------------------------------------------------------

// CreateAPIKey generates a new API key for a tenant. Returns the raw key (shown once).
func (s *Store) CreateAPIKey(ctx context.Context, tenantID, name string) (api.CreateAPIKeyResponse, error) {
	raw, err := generateToken(32)
	if err != nil {
		return api.CreateAPIKeyResponse{}, err
	}

	id := fmt.Sprintf("key-%d", time.Now().UnixNano())
	hash := hashToken(raw)

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO api_keys (id, tenant_id, name, key_hash)
		VALUES ($1, $2, $3, $4)
	`, id, tenantID, name, hash)
	if err != nil {
		return api.CreateAPIKeyResponse{}, err
	}

	return api.CreateAPIKeyResponse{
		APIKey: api.APIKey{
			ID:        id,
			TenantID:  tenantID,
			Name:      name,
			KeyPrefix: raw[:8],
		},
		RawKey: raw,
	}, nil
}

// ValidateAPIKey checks a raw key and returns the tenant ID if valid.
func (s *Store) ValidateAPIKey(ctx context.Context, raw string) (string, error) {
	hash := hashToken(raw)
	var tenantID string
	err := s.db.QueryRowContext(ctx, `
		SELECT tenant_id FROM api_keys WHERE key_hash = $1
	`, hash).Scan(&tenantID)
	if err == sql.ErrNoRows {
		return "", errors.New("invalid api key")
	}
	if err != nil {
		return "", err
	}
	// Update last used
	_, _ = s.db.ExecContext(ctx, `
		UPDATE api_keys SET last_used_at = NOW() WHERE key_hash = $1
	`, hash)
	return tenantID, nil
}

// ---------------------------------------------------------------------------
// Authentication — Enrollment Tokens
// ---------------------------------------------------------------------------

// CreateEnrollmentToken generates a single-use token for agent registration.
func (s *Store) CreateEnrollmentToken(ctx context.Context, tenantID, label string, expiresAt *time.Time) (string, error) {
	raw, err := generateToken(24)
	if err != nil {
		return "", err
	}

	id := fmt.Sprintf("enroll-%d", time.Now().UnixNano())
	hash := hashToken(raw)

	if expiresAt != nil {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO enrollment_tokens (id, tenant_id, token_hash, label, expires_at)
			VALUES ($1, $2, $3, $4, $5)
		`, id, tenantID, hash, label, expiresAt)
	} else {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO enrollment_tokens (id, tenant_id, token_hash, label)
			VALUES ($1, $2, $3, $4)
		`, id, tenantID, hash, label)
	}
	return raw, err
}

func (s *Store) consumeEnrollmentToken(ctx context.Context, tenantID, raw string) error {
	hash := hashToken(raw)
	var id string
	var used bool
	var expiresAt sql.NullTime
	var tokenTenantID string

	err := s.db.QueryRowContext(ctx, `
		SELECT id, tenant_id, used, expires_at
		FROM enrollment_tokens
		WHERE token_hash = $1
	`, hash).Scan(&id, &tokenTenantID, &used, &expiresAt)

	if err == sql.ErrNoRows {
		return errors.New("token not found")
	}
	if err != nil {
		return err
	}
	if used {
		return errors.New("token already used")
	}
	if tokenTenantID != tenantID {
		return errors.New("token does not belong to this tenant")
	}
	if expiresAt.Valid && expiresAt.Time.Before(time.Now()) {
		return errors.New("token expired")
	}

	_, err = s.db.ExecContext(ctx, `UPDATE enrollment_tokens SET used = TRUE WHERE id = $1`, id)
	return err
}

// ---------------------------------------------------------------------------
// Auth middleware helper
// ---------------------------------------------------------------------------

// RequireAPIKey returns the tenant ID from the Authorization header, or an error.
func (s *Store) RequireAPIKey(ctx context.Context, authHeader string) (string, error) {
	if authHeader == "" {
		return "", errors.New("missing Authorization header")
	}
	const prefix = "Bearer "
	if len(authHeader) <= len(prefix) {
		return "", errors.New("invalid Authorization header")
	}
	raw := authHeader[len(prefix):]
	return s.ValidateAPIKey(ctx, raw)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func generateToken(n int) (string, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func hashToken(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

// ---------------------------------------------------------------------------
// Scripts
// ---------------------------------------------------------------------------

type Script struct {
	ID          string `json:"id"`
	TenantID    string `json:"tenant_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Body        string `json:"body"`
	CreatedBy   string `json:"created_by"`
	CreatedAt   string `json:"created_at"`
}

func (s *Store) CreateScript(ctx context.Context, tenantID, name, description, body string) (Script, error) {
	id := fmt.Sprintf("script-%d", time.Now().UnixNano())
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO scripts (id, tenant_id, name, description, body)
		VALUES ($1, $2, $3, $4, $5)
	`, id, tenantID, name, description, body)
	if err != nil {
		return Script{}, err
	}
	return Script{ID: id, TenantID: tenantID, Name: name, Description: description, Body: body, CreatedBy: "dashboard"}, nil
}

func (s *Store) ListScripts(ctx context.Context, tenantID string) ([]Script, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, name, COALESCE(description,''), body, created_by, created_at::text
		FROM scripts WHERE tenant_id = $1 ORDER BY name
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var scripts []Script
	for rows.Next() {
		var sc Script
		if err := rows.Scan(&sc.ID, &sc.TenantID, &sc.Name, &sc.Description, &sc.Body, &sc.CreatedBy, &sc.CreatedAt); err != nil {
			return nil, err
		}
		scripts = append(scripts, sc)
	}
	return scripts, rows.Err()
}

func (s *Store) DeleteScript(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM scripts WHERE id = $1`, id)
	return err
}

// ---------------------------------------------------------------------------
// Alert Rules
// ---------------------------------------------------------------------------

type AlertRule struct {
	ID         string  `json:"id"`
	TenantID   string  `json:"tenant_id"`
	Name       string  `json:"name"`
	MetricName string  `json:"metric_name"`
	Operator   string  `json:"operator"`
	Threshold  float64 `json:"threshold"`
	DurationS  int     `json:"duration_seconds"`
	Enabled    bool    `json:"enabled"`
	CreatedAt  string  `json:"created_at"`
}

type AlertEvent struct {
	ID          int64   `json:"id"`
	RuleID      string  `json:"rule_id"`
	RuleName    string  `json:"rule_name"`
	AgentID     string  `json:"agent_id"`
	MetricValue float64 `json:"metric_value"`
	FiredAt     string  `json:"fired_at"`
	ResolvedAt  string  `json:"resolved_at,omitempty"`
}

func (s *Store) CreateAlertRule(ctx context.Context, r AlertRule) (AlertRule, error) {
	r.ID = fmt.Sprintf("rule-%d", time.Now().UnixNano())
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alert_rules (id, tenant_id, name, metric_name, operator, threshold, duration_s, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, r.ID, r.TenantID, r.Name, r.MetricName, r.Operator, r.Threshold, r.DurationS, r.Enabled)
	return r, err
}

func (s *Store) ListAlertRules(ctx context.Context, tenantID string) ([]AlertRule, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, name, metric_name, operator, threshold, duration_s, enabled, created_at::text
		FROM alert_rules WHERE tenant_id = $1 ORDER BY name
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []AlertRule
	for rows.Next() {
		var r AlertRule
		if err := rows.Scan(&r.ID, &r.TenantID, &r.Name, &r.MetricName, &r.Operator, &r.Threshold, &r.DurationS, &r.Enabled, &r.CreatedAt); err != nil {
			return nil, err
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (s *Store) DeleteAlertRule(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM alert_rules WHERE id = $1`, id)
	return err
}

func (s *Store) FireAlert(ctx context.Context, ruleID, agentID string, value float64) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO alert_events (rule_id, agent_id, metric_value) VALUES ($1, $2, $3)
	`, ruleID, agentID, value)
	return err
}

func (s *Store) ListAlertEvents(ctx context.Context, tenantID string, limit int) ([]AlertEvent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT ae.id, ae.rule_id, ar.name, ae.agent_id, ae.metric_value,
		       ae.fired_at::text, COALESCE(ae.resolved_at::text, '')
		FROM alert_events ae
		JOIN alert_rules ar ON ar.id = ae.rule_id
		WHERE ar.tenant_id = $1
		ORDER BY ae.fired_at DESC LIMIT $2
	`, tenantID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []AlertEvent
	for rows.Next() {
		var e AlertEvent
		if err := rows.Scan(&e.ID, &e.RuleID, &e.RuleName, &e.AgentID, &e.MetricValue, &e.FiredAt, &e.ResolvedAt); err != nil {
			return nil, err
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// EvaluateAlerts checks latest metric samples against all enabled rules and fires events.
func (s *Store) EvaluateAlerts(ctx context.Context) error {
	if s == nil {
		return nil
	}
	rules, err := s.ListAlertRules(ctx, "dev-tenant-001")
	if err != nil || len(rules) == 0 {
		return err
	}
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		rows, err := s.db.QueryContext(ctx, `
			SELECT agent_id, metric_value FROM metric_samples
			WHERE metric_name = $1
			ORDER BY collected_at DESC LIMIT 10
		`, rule.MetricName)
		if err != nil {
			continue
		}
		for rows.Next() {
			var agentID string
			var val float64
			if err := rows.Scan(&agentID, &val); err != nil {
				continue
			}
			triggered := false
			switch rule.Operator {
			case ">":
				triggered = val > rule.Threshold
			case "<":
				triggered = val < rule.Threshold
			case ">=":
				triggered = val >= rule.Threshold
			case "<=":
				triggered = val <= rule.Threshold
			}
			if triggered {
				_ = s.FireAlert(ctx, rule.ID, agentID, val)
			}
		}
		_ = rows.Close()
	}
	return nil
}
