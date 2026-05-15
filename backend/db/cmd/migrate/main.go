package main

import (
	"flag"
	"log"
	"os"

	"backend-db/internal/migrator"
)

func main() {
	var direction string
	var dryRun bool
	var dir string

	flag.StringVar(&direction, "direction", "up", "migration direction: up or down")
	flag.StringVar(&dir, "dir", "./migrations", "migrations directory")
	flag.BoolVar(&dryRun, "dry-run", false, "print migration order without executing")
	flag.Parse()

	cfg := migrator.Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		MigrationsDir: dir,
		DryRun:        dryRun,
	}

	var err error
	switch direction {
	case "up":
		err = migrator.RunUp(cfg)
	case "down":
		err = migrator.RunDown(cfg)
	default:
		log.Fatalf("unsupported direction: %s", direction)
	}

	if err != nil {
		log.Fatal(err)
	}
}
