package main

import (
	"log"
	"net/http"
	"os"

	"example.com/rmm-shared/httpjson"
	"example.com/rmm-shared/store"
	"command-service/internal/httpapi"
)

func main() {
	addr := envOrDefault("COMMAND_ADDR", ":8084")
	db, err := store.Open(os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	if db != nil {
		defer func() { _ = db.Close() }()
	}
	log.Printf("command service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, httpjson.WithCORS(httpapi.NewMux(db))))
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
