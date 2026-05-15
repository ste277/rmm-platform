package config

import (
	"errors"
	"os"
)

type Config struct {
	ServerURL string
	AgentID   string
	TenantID  string
	Hostname  string
	DevMode   bool
}

func Load() (Config, error) {
	cfg := Config{
		ServerURL: os.Getenv("RMM_SERVER_URL"),
		AgentID:   os.Getenv("RMM_AGENT_ID"),
		TenantID:  os.Getenv("RMM_TENANT_ID"),
		Hostname:  hostnameOrDefault(),
		DevMode:   os.Getenv("RMM_DEV_MODE") == "true",
	}

	if cfg.ServerURL == "" {
		cfg.ServerURL = "http://127.0.0.1:8080"
	}

	if cfg.AgentID == "" {
		cfg.AgentID = "dev-agent"
	}
	if cfg.TenantID == "" {
		cfg.TenantID = "dev-tenant"
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
