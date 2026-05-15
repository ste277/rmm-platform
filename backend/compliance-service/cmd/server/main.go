package main

import (
	"log"
	"net/http"
	"os"

	"compliance-service/internal/httpapi"
	"example.com/rmm-shared/store"
)

func main() {
	addr := envOrDefault("COMPLIANCE_ADDR", ":8085")
	db, err := store.Open(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	if db != nil {
		defer func() { _ = db.Close() }()
	}
	log.Printf("compliance service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, httpapi.NewMux(db)))
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
