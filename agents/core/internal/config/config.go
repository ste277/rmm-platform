package config

import (
	"errors"
	"os"
)

type Config struct {
	ServerURL       string
	AgentID         string
	TenantID        string
	Hostname        string
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
		// In production, TenantID is required; AgentID can come from registration
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
