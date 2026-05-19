package telemetry

import (
	"context"
	"log"
	"time"

	"example.com/rmm-shared/api"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
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
	ticker := time.NewTicker(60 * time.Second)
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
	log.Println("collecting telemetry")

	now := time.Now().Unix()
	tags := map[string]string{"agent_id": s.cfg.AgentID}
	var points []api.MetricPoint

	// CPU utilisation (non-blocking, returns average since last call)
	if pcts, err := cpu.PercentWithContext(ctx, 0, false); err == nil && len(pcts) > 0 {
		points = append(points, api.MetricPoint{
			Name:            "system.cpu.percent",
			Value:           pcts[0],
			CollectedAtUnix: now,
			Tags:            tags,
		})
	}

	// Memory
	if vm, err := mem.VirtualMemoryWithContext(ctx); err == nil {
		points = append(points,
			api.MetricPoint{
				Name:            "system.mem.used_bytes",
				Value:           float64(vm.Used),
				CollectedAtUnix: now,
				Tags:            tags,
			},
			api.MetricPoint{
				Name:            "system.mem.percent",
				Value:           vm.UsedPercent,
				CollectedAtUnix: now,
				Tags:            tags,
			},
		)
	}

	// Disk — root partition
	if du, err := disk.UsageWithContext(ctx, "/"); err == nil {
		diskTags := map[string]string{"agent_id": s.cfg.AgentID, "path": "/"}
		points = append(points,
			api.MetricPoint{
				Name:            "system.disk.used_bytes",
				Value:           float64(du.Used),
				CollectedAtUnix: now,
				Tags:            diskTags,
			},
			api.MetricPoint{
				Name:            "system.disk.percent",
				Value:           du.UsedPercent,
				CollectedAtUnix: now,
				Tags:            diskTags,
			},
		)
	}

	msg := api.IngestRequest{
		Type:     "metrics",
		AgentID:  s.cfg.AgentID,
		TenantID: s.cfg.TenantID,
		Points:   points,
	}

	if err := s.transport.Send(msg); err != nil {
		log.Printf("telemetry send error: %v", err)
	}
}
