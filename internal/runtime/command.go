package runtime

import (
	"context"
	"io"
	"os/exec"
)

type CommandRunner interface {
	Run(ctx context.Context, dir string, name string, args []string, stdout io.Writer, stderr io.Writer) error
}

type OSCommandRunner struct{}

func (OSCommandRunner) Run(ctx context.Context, dir string, name string, args []string, stdout io.Writer, stderr io.Writer) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	return cmd.Run()
}
