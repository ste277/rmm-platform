package compliance

import (
	"context"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"example.com/rmm-shared/api"
	"rmm-agent/internal/config"
	"rmm-agent/internal/transport"
)

// Service collects compliance findings on a schedule and reports them.
type Service struct {
	transport *transport.Client
	cfg       config.Config
}

func NewService(t *transport.Client, cfg config.Config) *Service {
	return &Service{transport: t, cfg: cfg}
}

func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	s.collectAndSend(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.collectAndSend(ctx)
		}
	}
}

func (s *Service) collectAndSend(_ context.Context) {
	log.Println("collecting compliance findings")

	findings := runChecks()

	overallStatus := "compliant"
	for _, f := range findings {
		if f.Status == string(StatusFailed) || f.Status == string(StatusMissing) {
			overallStatus = "non_compliant"
			break
		}
	}

	msg := api.IngestRequest{
		Type:     "compliance",
		AgentID:  s.cfg.AgentID,
		TenantID: s.cfg.TenantID,
		Hostname: s.cfg.Hostname,
		Status:   overallStatus,
		Findings: findings,
	}

	if err := s.transport.Send(msg); err != nil {
		log.Printf("compliance send error: %v", err)
	}
}

// runChecks executes all built-in compliance checks.
// Add new check functions here to extend coverage.
func runChecks() []api.ComplianceFinding {
	var findings []api.ComplianceFinding

	findings = append(findings, checkHostname())
	findings = append(findings, checkTempDirWritable())

	if runtime.GOOS == "linux" {
		findings = append(findings, checkSSHPasswordAuth())
	}

	return findings
}

// ---------------------------------------------------------------------------
// Built-in checks
// ---------------------------------------------------------------------------

func checkHostname() api.ComplianceFinding {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return api.ComplianceFinding{
			Category:   "system",
			ResourceID: "hostname",
			Status:     string(StatusFailed),
			Reason:     "hostname resolution failed",
			ActionHint: "check /etc/hostname or system network configuration",
		}
	}
	return api.ComplianceFinding{
		Category:   "system",
		ResourceID: "hostname",
		Status:     string(StatusInstalled),
		Reason:     "hostname resolved: " + hostname,
	}
}

func checkTempDirWritable() api.ComplianceFinding {
	tmpDir := os.TempDir()
	f, err := os.CreateTemp(tmpDir, "rmm-compliance-*")
	if err != nil {
		return api.ComplianceFinding{
			Category:   "filesystem",
			ResourceID: "tmp_writable",
			Status:     string(StatusFailed),
			Reason:     "temp dir not writable: " + err.Error(),
			ActionHint: "check permissions on " + tmpDir,
		}
	}
	_ = f.Close()
	_ = os.Remove(f.Name())
	return api.ComplianceFinding{
		Category:   "filesystem",
		ResourceID: "tmp_writable",
		Status:     string(StatusInstalled),
		Reason:     tmpDir + " is writable",
	}
}

func checkSSHPasswordAuth() api.ComplianceFinding {
	data, err := os.ReadFile("/etc/ssh/sshd_config")
	if err != nil {
		return api.ComplianceFinding{
			Category:   "security",
			ResourceID: "sshd_password_auth",
			Status:     string(StatusNeedsReview),
			Reason:     "could not read /etc/ssh/sshd_config",
			ActionHint: "ensure sshd is installed and config is readable",
		}
	}

	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if line == "PasswordAuthentication no" {
			return api.ComplianceFinding{
				Category:   "security",
				ResourceID: "sshd_password_auth",
				Status:     string(StatusInstalled),
				Reason:     "PasswordAuthentication is disabled",
			}
		}
	}

	return api.ComplianceFinding{
		Category:   "security",
		ResourceID: "sshd_password_auth",
		Status:     string(StatusFailed),
		Reason:     "PasswordAuthentication is not explicitly disabled",
		ActionHint: "set 'PasswordAuthentication no' in /etc/ssh/sshd_config and reload sshd",
	}
}
