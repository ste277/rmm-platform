package providers

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// ---------------------------------------------------------------------------
// macOS — Homebrew
// ---------------------------------------------------------------------------

type BrewProvider struct{}

func (p *BrewProvider) Name() string { return "homebrew" }

func (p *BrewProvider) DetectInstalled(ctx context.Context, pkg string) (string, error) {
	out, err := exec.CommandContext(ctx, "brew", "list", "--versions", pkg).Output()
	if err != nil {
		return "", fmt.Errorf("not installed: %w", err)
	}
	parts := strings.Fields(strings.TrimSpace(string(out)))
	if len(parts) < 2 {
		return "unknown", nil
	}
	return parts[1], nil
}

func (p *BrewProvider) Install(ctx context.Context, pkg string) (Result, error) {
	return runCmd(ctx, "brew", "install", pkg)
}

func (p *BrewProvider) Update(ctx context.Context, pkg string) (Result, error) {
	return runCmd(ctx, "brew", "upgrade", pkg)
}

func (p *BrewProvider) Uninstall(ctx context.Context, pkg string) (Result, error) {
	return runCmd(ctx, "brew", "uninstall", pkg)
}

// ---------------------------------------------------------------------------
// Linux — APT / dpkg
// ---------------------------------------------------------------------------

type AptProvider struct{}

func (p *AptProvider) Name() string { return "apt" }

func (p *AptProvider) DetectInstalled(ctx context.Context, pkg string) (string, error) {
	out, err := exec.CommandContext(ctx, "dpkg-query", "-W", "-f=${Version}", pkg).Output()
	if err != nil {
		return "", fmt.Errorf("not installed: %w", err)
	}
	ver := strings.TrimSpace(string(out))
	if ver == "" {
		return "", fmt.Errorf("not installed")
	}
	return ver, nil
}

func (p *AptProvider) Install(ctx context.Context, pkg string) (Result, error) {
	return runCmd(ctx, "apt-get", "install", "-y", pkg)
}

func (p *AptProvider) Update(ctx context.Context, pkg string) (Result, error) {
	return runCmd(ctx, "apt-get", "install", "-y", "--only-upgrade", pkg)
}

func (p *AptProvider) Uninstall(ctx context.Context, pkg string) (Result, error) {
	return runCmd(ctx, "apt-get", "remove", "-y", pkg)
}

// ---------------------------------------------------------------------------
// Windows — winget
// ---------------------------------------------------------------------------

type WingetProvider struct{}

func (p *WingetProvider) Name() string { return "winget" }

func (p *WingetProvider) DetectInstalled(ctx context.Context, pkg string) (string, error) {
	out, err := exec.CommandContext(ctx, "winget", "list", "--id", pkg, "--exact").Output()
	if err != nil {
		return "", fmt.Errorf("not installed: %w", err)
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, pkg) {
			fields := strings.Fields(line)
			if len(fields) >= 3 {
				return fields[2], nil // version is third column
			}
		}
	}
	return "unknown", nil
}

func (p *WingetProvider) Install(ctx context.Context, pkg string) (Result, error) {
	return runCmd(ctx, "winget", "install", "--id", pkg, "--exact", "--silent")
}

func (p *WingetProvider) Update(ctx context.Context, pkg string) (Result, error) {
	return runCmd(ctx, "winget", "upgrade", "--id", pkg, "--exact", "--silent")
}

func (p *WingetProvider) Uninstall(ctx context.Context, pkg string) (Result, error) {
	return runCmd(ctx, "winget", "uninstall", "--id", pkg, "--exact", "--silent")
}

// ---------------------------------------------------------------------------
// Noop — fallback for unsupported platforms
// ---------------------------------------------------------------------------

type NoopProvider struct{}

func (p *NoopProvider) Name() string { return "noop" }

func (p *NoopProvider) DetectInstalled(_ context.Context, _ string) (string, error) {
	return "", fmt.Errorf("platform not supported")
}

func (p *NoopProvider) Install(_ context.Context, _ string) (Result, error) {
	return Result{ErrorCode: "unsupported"}, fmt.Errorf("platform not supported")
}

func (p *NoopProvider) Update(_ context.Context, _ string) (Result, error) {
	return Result{ErrorCode: "unsupported"}, fmt.Errorf("platform not supported")
}

func (p *NoopProvider) Uninstall(_ context.Context, _ string) (Result, error) {
	return Result{ErrorCode: "unsupported"}, fmt.Errorf("platform not supported")
}

// ---------------------------------------------------------------------------
// Shared helper
// ---------------------------------------------------------------------------

func runCmd(ctx context.Context, name string, args ...string) (Result, error) {
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return Result{
			Success:   false,
			ErrorCode: "exec_failed",
			Message:   strings.TrimSpace(string(out)),
		}, err
	}
	return Result{
		Success: true,
		Message: strings.TrimSpace(string(out)),
	}, nil
}
