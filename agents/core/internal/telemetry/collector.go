package telemetry

import (
	"context"
	"log"
	"runtime"
	"time"

	"rmm-agent/internal/transport"
)

type Service struct {
	transport *transport.Client
}

func NewService(t *transport.Client) *Service {
	return &Service{transport: t}
}

func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
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
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	log.Println("collecting telemetry")
	_ = s.transport.Send(map[string]any{
		"type": "metrics",
		"points": []map[string]any{
			{
				"name":              "agent.go.goroutines",
				"value":             runtime.NumGoroutine(),
				"collected_at_unix": time.Now().Unix(),
			},
			{
				"name":              "agent.go.heap_alloc_bytes",
				"value":             mem.HeapAlloc,
				"collected_at_unix": time.Now().Unix(),
			},
		},
	})
}
