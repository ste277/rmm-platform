package providers

import "context"

type Result struct {
	Success      bool
	InstalledVer string
	ErrorCode    string
	Message      string
}

type Provider interface {
	Name() string
	DetectInstalled(ctx context.Context, pkg string) (string, error)
	Install(ctx context.Context, pkg string) (Result, error)
	Update(ctx context.Context, pkg string) (Result, error)
	Uninstall(ctx context.Context, pkg string) (Result, error)
}
