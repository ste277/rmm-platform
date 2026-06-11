package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"example.com/rmm-shared/httpjson"
	"example.com/rmm-shared/store"
	"gateway-service/internal/httpapi"
)

func main() {
	addr := envOrDefault("GATEWAY_ADDR", ":8080")
	db, err := store.Open(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	if db != nil {
		defer func() { _ = db.Close() }()
		// Evaluate alert rules every 60s
		go func() {
			ticker := time.NewTicker(60 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				if err := db.EvaluateAlerts(context.Background()); err != nil {
					log.Printf("alert evaluation error: %v", err)
				}
			}
		}()
	}
	log.Printf("gateway service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, httpjson.WithCORS(httpapi.NewMux(db))))
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
