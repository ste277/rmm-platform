package inventory

import (
	"context"
	"log"
	"runtime"
	"time"

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
	ticker := time.NewTicker(10 * time.Minute)
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
	log.Println("collecting inventory")
	_ = s.transport.Send(map[string]any{
		"type":         "inventory",
		"hostname":     s.cfg.Hostname,
		"os_family":    runtime.GOOS,
		"os_version":   "dev-local",
		"architecture": runtime.GOARCH,
		"software": []map[string]string{
			{
				"name":      "rmm-agent",
				"version":   "0.1.0-dev",
				"publisher": "local",
				"source":    "scaffold",
			},
		},
	})
}
