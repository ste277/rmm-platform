package config

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type Config struct {
	ServerURL       string
	AgentID         string
	TenantID        string
	Hostname        string
	OSVersion       string
	EnrollmentToken string
	DevMode         bool
}

func Load() (Config, error) {
	cfg := Config{
		ServerURL:       os.Getenv("RMM_SERVER_URL"),
		AgentID:         os.Getenv("RMM_AGENT_ID"),
		TenantID:        os.Getenv("RMM_TENANT_ID"),
		EnrollmentToken: os.Getenv("RMM_ENROLLMENT_TOKEN"),
		Hostname:        hostnameOrDefault(),
		OSVersion:       detectOSVersion(),
		DevMode:         os.Getenv("RMM_DEV_MODE") == "true",
	}

	if cfg.ServerURL == "" {
		cfg.ServerURL = "http://127.0.0.1:8080"
	}

	if cfg.DevMode {
		if cfg.AgentID == "" {
			cfg.AgentID = "dev-agent"
		}
		if cfg.TenantID == "" {
			cfg.TenantID = "dev-tenant"
		}
	} else {
		if cfg.TenantID == "" {
			return Config{}, errors.New("RMM_TENANT_ID is required in production mode")
		}
	}

	if cfg.Hostname == "" {
		return Config{}, errors.New("hostname resolution failed")
	}

	return cfg, nil
}

func hostnameOrDefault() string {
	name, err := os.Hostname()
	if err != nil || name == "" {
		return "unknown-host"
	}
	return name
}

// detectOSVersion returns a human-readable OS version string.
func detectOSVersion() string {
	switch runtime.GOOS {
	case "darwin":
		return darwinVersion()
	case "linux":
		return linuxVersion()
	case "windows":
		return windowsVersion()
	default:
		return fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func darwinVersion() string {
	out, err := exec.Command("sw_vers", "-productVersion").Output()
	if err != nil {
		return "macOS unknown"
	}
	return "macOS " + strings.TrimSpace(string(out))
}

func linuxVersion() string {
	// Try /etc/os-release first
	data, err := os.ReadFile("/etc/os-release")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				val := strings.TrimPrefix(line, "PRETTY_NAME=")
				return strings.Trim(val, `"`)
			}
		}
	}
	// Fallback: uname
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return "Linux unknown"
	}
	return "Linux " + strings.TrimSpace(string(out))
}

func windowsVersion() string {
	out, err := exec.Command("cmd", "/c", "ver").Output()
	if err != nil {
		return "Windows unknown"
	}
	return strings.TrimSpace(string(out))
}
