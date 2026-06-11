package compliance

import (
	"context"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"example.com/rmm-shared/api"
	"rmm-agent/internal/config"
	"rmm-agent/internal/transport"
)

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
	s.collectAndSend()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.collectAndSend()
		}
	}
}

func (s *Service) collectAndSend() {
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

func runChecks() []api.ComplianceFinding {
	var findings []api.ComplianceFinding

	// Universal checks
	findings = append(findings, checkHostname())
	findings = append(findings, checkTempDirWritable())

	// Platform-specific checks
	switch runtime.GOOS {
	case "darwin":
		findings = append(findings, checkFirewallMac())
		findings = append(findings, checkFileVault())
		findings = append(findings, checkScreenLockMac())
		findings = append(findings, checkGatekeeperMac())
	case "linux":
		findings = append(findings, checkSSHPasswordAuth())
		findings = append(findings, checkFirewallLinux())
		findings = append(findings, checkWorldWritableDirs())
	}

	return findings
}

// ── Universal ────────────────────────────────────────────────────────────────

func checkHostname() api.ComplianceFinding {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return api.ComplianceFinding{
			Category: "system", ResourceID: "hostname",
			Status: string(StatusFailed), Reason: "hostname resolution failed",
			ActionHint: "check /etc/hostname or system network configuration",
		}
	}
	return api.ComplianceFinding{
		Category: "system", ResourceID: "hostname",
		Status: string(StatusInstalled), Reason: "hostname resolved: " + hostname,
	}
}

func checkTempDirWritable() api.ComplianceFinding {
	tmpDir := os.TempDir()
	f, err := os.CreateTemp(tmpDir, "rmm-compliance-*")
	if err != nil {
		return api.ComplianceFinding{
			Category: "filesystem", ResourceID: "tmp_writable",
			Status: string(StatusFailed), Reason: "temp dir not writable: " + err.Error(),
			ActionHint: "check permissions on " + tmpDir,
		}
	}
	_ = f.Close()
	_ = os.Remove(f.Name())
	return api.ComplianceFinding{
		Category: "filesystem", ResourceID: "tmp_writable",
		Status: string(StatusInstalled), Reason: tmpDir + " is writable",
	}
}

// ── macOS ────────────────────────────────────────────────────────────────────

func checkFirewallMac() api.ComplianceFinding {
	out, err := exec.Command(
		"/usr/libexec/ApplicationFirewall/socketfilterfw", "--getglobalstate",
	).CombinedOutput()
	if err != nil {
		return api.ComplianceFinding{
			Category: "security", ResourceID: "firewall",
			Status: string(StatusNeedsReview), Reason: "could not query firewall state",
		}
	}
	s := strings.ToLower(string(out))
	if strings.Contains(s, "enabled") {
		return api.ComplianceFinding{
			Category: "security", ResourceID: "firewall",
			Status: string(StatusInstalled), Reason: "macOS firewall is enabled",
		}
	}
	return api.ComplianceFinding{
		Category: "security", ResourceID: "firewall",
		Status: string(StatusFailed), Reason: "macOS firewall is disabled",
		ActionHint: "System Settings → Network → Firewall → Turn On",
	}
}

func checkFileVault() api.ComplianceFinding {
	out, err := exec.Command("fdesetup", "status").CombinedOutput()
	if err != nil {
		return api.ComplianceFinding{
			Category: "security", ResourceID: "disk_encryption",
			Status: string(StatusNeedsReview), Reason: "could not query FileVault status",
		}
	}
	s := strings.ToLower(string(out))
	if strings.Contains(s, "filevault is on") {
		return api.ComplianceFinding{
			Category: "security", ResourceID: "disk_encryption",
			Status: string(StatusInstalled), Reason: "FileVault is enabled",
		}
	}
	return api.ComplianceFinding{
		Category: "security", ResourceID: "disk_encryption",
		Status: string(StatusFailed), Reason: "FileVault is disabled",
		ActionHint: "System Settings → Privacy & Security → FileVault → Turn On",
	}
}

func checkScreenLockMac() api.ComplianceFinding {
	out, err := exec.Command(
		"osascript", "-e",
		`tell application "System Events" to get value of slider 1 of group 1 of tab group 1 of window 1 of application process "System Preferences"`,
	).CombinedOutput()
	// osascript may fail if System Preferences isn't open — use defaults read instead
	_ = out
	_ = err

	out2, err2 := exec.Command(
		"defaults", "read", "com.apple.screensaver", "askForPasswordDelay",
	).CombinedOutput()
	if err2 != nil {
		return api.ComplianceFinding{
			Category: "security", ResourceID: "screen_lock",
			Status: string(StatusNeedsReview), Reason: "could not read screen lock setting",
		}
	}
	delay := strings.TrimSpace(string(out2))
	if delay == "0" {
		return api.ComplianceFinding{
			Category: "security", ResourceID: "screen_lock",
			Status: string(StatusInstalled), Reason: "screen lock requires password immediately",
		}
	}
	return api.ComplianceFinding{
		Category: "security", ResourceID: "screen_lock",
		Status: string(StatusFailed),
		Reason:     "screen lock password delay is " + delay + "s (should be 0)",
		ActionHint: "System Settings → Lock Screen → Require password: immediately",
	}
}

func checkGatekeeperMac() api.ComplianceFinding {
	out, err := exec.Command("spctl", "--status").CombinedOutput()
	if err != nil {
		return api.ComplianceFinding{
			Category: "security", ResourceID: "gatekeeper",
			Status: string(StatusNeedsReview), Reason: "could not query Gatekeeper status",
		}
	}
	if strings.Contains(string(out), "assessments enabled") {
		return api.ComplianceFinding{
			Category: "security", ResourceID: "gatekeeper",
			Status: string(StatusInstalled), Reason: "Gatekeeper is enabled",
		}
	}
	return api.ComplianceFinding{
		Category: "security", ResourceID: "gatekeeper",
		Status: string(StatusFailed), Reason: "Gatekeeper is disabled",
		ActionHint: "run: sudo spctl --master-enable",
	}
}

// ── Linux ────────────────────────────────────────────────────────────────────

func checkSSHPasswordAuth() api.ComplianceFinding {
	data, err := os.ReadFile("/etc/ssh/sshd_config")
	if err != nil {
		return api.ComplianceFinding{
			Category: "security", ResourceID: "sshd_password_auth",
			Status: string(StatusNeedsReview), Reason: "could not read /etc/ssh/sshd_config",
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
				Category: "security", ResourceID: "sshd_password_auth",
				Status: string(StatusInstalled), Reason: "PasswordAuthentication is disabled",
			}
		}
	}
	return api.ComplianceFinding{
		Category: "security", ResourceID: "sshd_password_auth",
		Status: string(StatusFailed), Reason: "PasswordAuthentication is not explicitly disabled",
		ActionHint: "set 'PasswordAuthentication no' in /etc/ssh/sshd_config and reload sshd",
	}
}

func checkFirewallLinux() api.ComplianceFinding {
	// Try ufw first, then iptables
	out, err := exec.Command("ufw", "status").CombinedOutput()
	if err == nil {
		if strings.Contains(string(out), "Status: active") {
			return api.ComplianceFinding{
				Category: "security", ResourceID: "firewall",
				Status: string(StatusInstalled), Reason: "ufw firewall is active",
			}
		}
		return api.ComplianceFinding{
			Category: "security", ResourceID: "firewall",
			Status: string(StatusFailed), Reason: "ufw firewall is inactive",
			ActionHint: "run: sudo ufw enable",
		}
	}
	return api.ComplianceFinding{
		Category: "security", ResourceID: "firewall",
		Status: string(StatusNeedsReview), Reason: "could not determine firewall state (ufw not found)",
		ActionHint: "install ufw: apt install ufw && ufw enable",
	}
}

func checkWorldWritableDirs() api.ComplianceFinding {
	out, err := exec.Command("find", "/tmp", "-maxdepth", "1", "-perm", "-o+w", "-type", "d").CombinedOutput()
	if err != nil {
		return api.ComplianceFinding{
			Category: "filesystem", ResourceID: "world_writable_dirs",
			Status: string(StatusNeedsReview), Reason: "could not scan for world-writable dirs",
		}
	}
	dirs := strings.TrimSpace(string(out))
	if dirs == "" || dirs == "/tmp" {
		return api.ComplianceFinding{
			Category: "filesystem", ResourceID: "world_writable_dirs",
			Status: string(StatusInstalled), Reason: "no unexpected world-writable directories found",
		}
	}
	return api.ComplianceFinding{
		Category: "filesystem", ResourceID: "world_writable_dirs",
		Status: string(StatusFailed), Reason: "world-writable directories found: " + dirs,
		ActionHint: "review and restrict permissions with chmod o-w",
	}
}
