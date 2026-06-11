package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"broker-service/internal/httpapi"
	"example.com/rmm-shared/httpjson"
	"example.com/rmm-shared/store"
)

func main() {
	addr := envOrDefault("BROKER_ADDR", ":8081")
	db, err := store.Open(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	if db != nil {
		defer func() { _ = db.Close() }()
		// Background goroutine: mark agents offline if no heartbeat for 90s
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				if err := db.MarkAgentsOffline(context.Background(), 90*time.Second); err != nil {
					log.Printf("offline sweep error: %v", err)
				}
			}
		}()
	}
	log.Printf("broker service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, httpjson.WithCORS(httpapi.NewMux(db))))
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
