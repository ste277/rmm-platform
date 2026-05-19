package inventory

import (
	"context"
	"log"
	"runtime"
	"time"

	"example.com/rmm-shared/api"
	"rmm-agent/internal/config"
	"rmm-agent/internal/software/providers"
	"rmm-agent/internal/transport"
)

type Service struct {
	transport *transport.Client
	cfg       config.Config
	provider  providers.Provider
}

func NewService(t *transport.Client, cfg config.Config) *Service {
	return &Service{
		transport: t,
		cfg:       cfg,
		provider:  providers.ForPlatform(),
	}
}

func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
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

func (s *Service) collectAndSend(ctx context.Context) {
	log.Println("collecting inventory")

	msg := api.IngestRequest{
		Type:         "inventory",
		AgentID:      s.cfg.AgentID,
		TenantID:     s.cfg.TenantID,
		Hostname:     s.cfg.Hostname,
		OSFamily:     runtime.GOOS,
		Architecture: runtime.GOARCH,
		Software:     s.collectSoftware(ctx),
	}

	if err := s.transport.Send(msg); err != nil {
		log.Printf("inventory send error: %v", err)
	}
}

// collectSoftware queries the platform provider for known packages.
// Extend knownPackages to track more software.
func (s *Service) collectSoftware(ctx context.Context) []map[string]string {
	knownPackages := []string{
		"git", "curl", "wget", "docker", "node", "python3", "go",
	}

	var results []map[string]string
	for _, pkg := range knownPackages {
		ver, err := s.provider.DetectInstalled(ctx, pkg)
		if err != nil {
			continue // not installed or undetectable — skip silently
		}
		results = append(results, map[string]string{
			"name":      pkg,
			"version":   ver,
			"publisher": s.provider.Name(),
			"source":    "detected",
		})
	}

	// Always include the agent itself
	results = append(results, map[string]string{
		"name":      "rmm-agent",
		"version":   "0.1.0-dev",
		"publisher": "local",
		"source":    "self",
	})

	return results
}
