package migrator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

type Config struct {
	DatabaseURL string
	MigrationsDir string
	DryRun bool
}

func RunUp(cfg Config) error {
	files, err := migrationFiles(cfg.MigrationsDir, ".up.sql")
	if err != nil {
		return err
	}
	for _, file := range files {
		if err := runFile(cfg, file); err != nil {
			return err
		}
	}
	return nil
}

func RunDown(cfg Config) error {
	files, err := migrationFiles(cfg.MigrationsDir, ".down.sql")
	if err != nil {
		return err
	}
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	for _, file := range files {
		if err := runFile(cfg, file); err != nil {
			return err
		}
	}
	return nil
}

func migrationFiles(dir, suffix string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}
		files = append(files, filepath.Join(dir, entry.Name()))
	}
	sort.Strings(files)
	return files, nil
}

func runFile(cfg Config, path string) error {
	if cfg.DryRun {
		fmt.Printf("DRY RUN: %s\n", path)
		return nil
	}
	if cfg.DatabaseURL == "" {
		return fmt.Errorf("DATABASE_URL is required unless DRY_RUN=true")
	}

	cmd := exec.Command("psql", cfg.DatabaseURL, "-v", "ON_ERROR_STOP=1", "-f", path)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
