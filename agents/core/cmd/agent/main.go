package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"rmm-agent/internal/app"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	a, err := app.New()
	if err != nil {
		log.Fatalf("init agent: %v", err)
	}

	if err := a.Run(ctx); err != nil {
		log.Fatalf("run agent: %v", err)
	}
}
