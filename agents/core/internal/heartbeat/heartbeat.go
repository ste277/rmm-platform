package heartbeat

import (
	"context"
	"log"
	"runtime"
	"time"

	"example.com/rmm-shared/api"
	"rmm-agent/internal/config"
	"rmm-agent/internal/transport"
)

const agentVersion = "0.1.0-dev"

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

	mode := "managed"
	if s.cfg.DevMode {
		mode = "dev"
	}

	msg := api.IngestRequest{
		Type:      "heartbeat",
		AgentID:   s.cfg.AgentID,
		TenantID:  s.cfg.TenantID,
		Hostname:  s.cfg.Hostname,
		OSFamily:  runtime.GOOS,
		OSVersion: runtime.GOARCH,
		Payload: map[string]any{
			"agent_version": agentVersion,
			"labels": map[string]string{
				"mode": mode,
			},
		},
	}

	if err := s.transport.Send(msg); err != nil {
		log.Printf("heartbeat send error: %v", err)
	}
}
