package app

import (
	"context"
	"log"
	"time"

	"rmm-agent/internal/config"
	"rmm-agent/internal/heartbeat"
	"rmm-agent/internal/inventory"
	"rmm-agent/internal/telemetry"
	"rmm-agent/internal/transport"
)

type App struct {
	cfg       config.Config
	transport *transport.Client
	heartbeat *heartbeat.Service
	inventory *inventory.Service
	telemetry *telemetry.Service
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	t := transport.NewClient(cfg.ServerURL)

	return &App{
		cfg:       cfg,
		transport: t,
		heartbeat: heartbeat.New(t, cfg),
		inventory: inventory.NewService(t, cfg),
		telemetry: telemetry.NewService(t),
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	log.Printf("agent starting: agent_id=%s tenant_id=%s dev_mode=%t", a.cfg.AgentID, a.cfg.TenantID, a.cfg.DevMode)

	if err := a.transport.Connect(ctx); err != nil {
		return err
	}

	a.heartbeat.SendNow()
	go a.heartbeat.Start(ctx)
	go a.inventory.Start(ctx)
	go a.telemetry.Start(ctx)

	<-ctx.Done()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.transport.Close(shutdownCtx)
}
