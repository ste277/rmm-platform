package app

import (
	"context"
	"log"
	"time"

	"rmm-agent/internal/command"
	"rmm-agent/internal/compliance"
	"rmm-agent/internal/config"
	"rmm-agent/internal/heartbeat"
	"rmm-agent/internal/inventory"
	"rmm-agent/internal/registration"
	"rmm-agent/internal/telemetry"
	"rmm-agent/internal/transport"
)

type App struct {
	cfg        config.Config
	transport  *transport.Client
	heartbeat  *heartbeat.Service
	inventory  *inventory.Service
	telemetry  *telemetry.Service
	compliance *compliance.Service
	commands   *command.Receiver
}

func New() (*App, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	// Registration: enrol with the platform to get AgentID + BrokerURL.
	// Skipped in dev mode.
	if !cfg.DevMode && (cfg.AgentID == "" || cfg.AgentID == "dev-agent") {
		regClient := registration.NewClient(cfg.ServerURL)
		resp, err := regClient.Enrol(
			context.Background(),
			cfg.TenantID,
			cfg.EnrollmentToken,
			cfg.Hostname,
		)
		if err != nil {
			return nil, err
		}
		log.Printf("registered: agent_id=%s broker_url=%s", resp.AgentID, resp.BrokerURL)
		cfg.AgentID = resp.AgentID
		cfg.ServerURL = resp.BrokerURL
	}

	t := transport.NewClient(cfg.ServerURL, cfg.AgentID)

	hb := heartbeat.New(t, cfg)

	// Send a heartbeat immediately whenever the WebSocket reconnects
	// so the broker knows the agent is back online without waiting 30s.
	t.SetOnReconnect(func() {
		log.Println("reconnected — sending immediate heartbeat")
		hb.SendNow()
	})

	return &App{
		cfg:        cfg,
		transport:  t,
		heartbeat:  hb,
		inventory:  inventory.NewService(t, cfg),
		telemetry:  telemetry.NewService(t, cfg),
		compliance: compliance.NewService(t, cfg),
		commands:   command.NewReceiver(t, cfg.AgentID),
	}, nil
}

func (a *App) Run(ctx context.Context) error {
	log.Printf("agent starting: agent_id=%s tenant_id=%s os_version=%s dev_mode=%t",
		a.cfg.AgentID, a.cfg.TenantID, a.cfg.OSVersion, a.cfg.DevMode)

	if err := a.transport.Connect(ctx); err != nil {
		return err
	}

	// Immediate first heartbeat
	a.heartbeat.SendNow()

	go a.heartbeat.Start(ctx)
	go a.inventory.Start(ctx)
	go a.telemetry.Start(ctx)
	go a.compliance.Start(ctx)
	go a.commands.Start(ctx)

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return a.transport.Close(shutdownCtx)
}
