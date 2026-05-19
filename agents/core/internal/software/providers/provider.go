package providers

import (
	"context"
	"runtime"
)

// Result is returned by mutating provider operations.
type Result struct {
	Success      bool
	InstalledVer string
	ErrorCode    string
	Message      string
}

// Provider is the cross-platform package management interface.
// Implement this for each target OS package manager.
type Provider interface {
	Name() string
	DetectInstalled(ctx context.Context, pkg string) (string, error)
	Install(ctx context.Context, pkg string) (Result, error)
	Update(ctx context.Context, pkg string) (Result, error)
	Uninstall(ctx context.Context, pkg string) (Result, error)
}

// ForPlatform returns the best available provider for the current OS.
func ForPlatform() Provider {
	switch runtime.GOOS {
	case "darwin":
		return &BrewProvider{}
	case "linux":
		return &AptProvider{}
	case "windows":
		return &WingetProvider{}
	default:
		return &NoopProvider{}
	}
}
