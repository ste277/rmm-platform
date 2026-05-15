package heartbeat

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

func New(t *transport.Client, cfg config.Config) *Service {
	return &Service{transport: t, cfg: cfg}
}

func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	s.SendNow()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.SendNow()
		}
	}
}

func (s *Service) SendNow() {
	log.Println("heartbeat tick")
	_ = s.transport.Send(map[string]any{
		"type":          "heartbeat",
		"agent_id":      s.cfg.AgentID,
		"tenant_id":     s.cfg.TenantID,
		"hostname":      s.cfg.Hostname,
		"agent_version": "0.1.0-dev",
		"os_family":     runtime.GOOS,
		"os_version":    runtime.GOARCH,
		"labels": map[string]string{
			"mode": map[bool]string{true: "dev", false: "managed"}[s.cfg.DevMode],
		},
	})
}
