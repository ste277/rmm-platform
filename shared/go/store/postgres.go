package store

import (
	"context"
	"database/sql"
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
	default:
		return nil
	}
}

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
			tenant_id = EXCLUDED.tenant_id,
			hostname = EXCLUDED.hostname,
			os_family = EXCLUDED.os_family,
			os_version = EXCLUDED.os_version,
			architecture = EXCLUDED.architecture,
			status = 'online',
			last_seen_at = NOW()
	`, req.AgentID, req.TenantID, req.Hostname, req.OSFamily, req.OSVersion, req.Architecture)
	return err
}

func (s *Store) ListAgents(ctx context.Context) ([]api.AgentSummary, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, tenant_id, hostname, os_family, os_version, architecture, status, COALESCE(last_seen_at::text, '')
		FROM agents
		ORDER BY hostname
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var agents []api.AgentSummary
	for rows.Next() {
		var agent api.AgentSummary
		if err := rows.Scan(&agent.ID, &agent.TenantID, &agent.Hostname, &agent.OSFamily, &agent.OSVersion, &agent.Architecture, &agent.Status, &agent.LastSeenAt); err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, rows.Err()
}

func (s *Store) RegisterAgent(ctx context.Context, req api.RegistrationRequest) (api.RegistrationResponse, error) {
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
			os_family = EXCLUDED.os_family,
			os_version = EXCLUDED.os_version,
			architecture = EXCLUDED.architecture,
			status = 'online',
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

	for _, software := range rec.Software {
		_, err = s.db.ExecContext(ctx, `
			INSERT INTO installed_software (agent_id, name, version, publisher, source)
			VALUES ($1, $2, $3, $4, $5)
		`, rec.AgentID, software["name"], software["version"], software["publisher"], software["source"])
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListInventory(ctx context.Context) ([]api.InventoryRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, hostname, os_family, os_version, architecture
		FROM agents
		ORDER BY hostname
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

func (s *Store) CreateCommand(ctx context.Context, req api.CommandCreateRequest) (api.CommandResponse, error) {
	commandID := fmt.Sprintf("cmd-%d", time.Now().UnixNano())
	payload, err := json.Marshal(req)
	if err != nil {
		return api.CommandResponse{}, err
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO commands (id, tenant_id, agent_id, command_type, payload, status, requested_by)
		VALUES ($1, $2, $3, $4, $5::jsonb, 'queued', 'local-dev')
	`, commandID, "dev-tenant-001", req.AgentID, req.CommandType, string(payload))
	if err != nil {
		return api.CommandResponse{}, err
	}
	return api.CommandResponse{CommandID: commandID, Status: "queued"}, nil
}

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

func (s *Store) ListCommands(ctx context.Context) ([]api.CommandRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, agent_id, command_type, status, created_at::text
		FROM commands
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []api.CommandRecord
	for rows.Next() {
		var rec api.CommandRecord
		if err := rows.Scan(&rec.CommandID, &rec.AgentID, &rec.CommandType, &rec.Status, &rec.CreatedAt); err != nil {
			return nil, err
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

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
		SELECT id, agent_id, status, generated_at::text
		FROM compliance_reports
		ORDER BY generated_at DESC
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
		var finding api.ComplianceFinding
		if err := rows.Scan(&finding.Category, &finding.ResourceID, &finding.Status, &finding.Reason, &finding.ActionHint); err != nil {
			return nil, err
		}
		findings = append(findings, finding)
	}
	return findings, rows.Err()
}
