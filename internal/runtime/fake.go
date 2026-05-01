package runtime

import (
	"context"
	"fmt"
	"io"
)

type FakeRuntime struct {
	EnsureFunc        func(ctx context.Context, workspaceRoot string) error
	ExecFunc          func(ctx context.Context, opts ExecOptions) error
	StopProcessFunc   func(ctx context.Context, workspaceRoot string, service string) error
	StartProcessFunc  func(ctx context.Context, workspaceRoot string, service string, command string) error
	ServiceStatusFunc func(ctx context.Context, workspaceRoot string, service string) (ServiceStatus, error)
}

func (f *FakeRuntime) Ensure(ctx context.Context, workspaceRoot string) error {
	if f.EnsureFunc != nil {
		return f.EnsureFunc(ctx, workspaceRoot)
	}
	return nil
}

func (f *FakeRuntime) Exec(ctx context.Context, opts ExecOptions) error {
	if f.ExecFunc != nil {
		return f.ExecFunc(ctx, opts)
	}
	return nil
}

func (f *FakeRuntime) StopProcess(ctx context.Context, workspaceRoot string, service string) error {
	if f.StopProcessFunc != nil {
		return f.StopProcessFunc(ctx, workspaceRoot, service)
	}
	return nil
}

func (f *FakeRuntime) StartProcess(ctx context.Context, workspaceRoot string, service string, command string) error {
	if f.StartProcessFunc != nil {
		return f.StartProcessFunc(ctx, workspaceRoot, service, command)
	}
	return nil
}

func (f *FakeRuntime) ServiceStatus(ctx context.Context, workspaceRoot string, service string) (ServiceStatus, error) {
	if f.ServiceStatusFunc != nil {
		return f.ServiceStatusFunc(ctx, workspaceRoot, service)
	}
	return ServiceStatus{Container: "running", Process: "running"}, nil
}

type FakeCommandRunner struct {
	RunFunc func(ctx context.Context, dir string, name string, args []string, stdout io.Writer, stderr io.Writer) error
}

func (f *FakeCommandRunner) Run(ctx context.Context, dir string, name string, args []string, stdout io.Writer, stderr io.Writer) error {
	if f.RunFunc != nil {
		return f.RunFunc(ctx, dir, name, args, stdout, stderr)
	}
	return fmt.Errorf("FakeCommandRunner: unexpected call %s %v", name, args)
}
