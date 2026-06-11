package main

import (
	"log"
	"net/http"
	"os"

	"example.com/rmm-shared/httpjson"
	"example.com/rmm-shared/store"
	"compliance-service/internal/httpapi"
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
	log.Fatal(http.ListenAndServe(addr, httpjson.WithCORS(httpapi.NewMux(db))))
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
