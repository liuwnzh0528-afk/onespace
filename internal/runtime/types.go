package runtime

import (
	"context"
	"io"
)

type ExecOptions struct {
	WorkspaceRoot string
	Service       string
	Command       string
	Stdout        io.Writer
	Stderr        io.Writer
}

type Runtime interface {
	Ensure(ctx context.Context, workspaceRoot string) error
	Exec(ctx context.Context, opts ExecOptions) error
	StopProcess(ctx context.Context, workspaceRoot string, service string) error
	StartProcess(ctx context.Context, workspaceRoot string, service string, command string) error
	ServiceStatus(ctx context.Context, workspaceRoot string, service string) (ServiceStatus, error)
}

type ServiceStatus struct {
	Container string `json:"container"`
	Process   string `json:"process"`
}
