package runtime

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSupervisorExecsManagedCommand(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "deploy", "runner", "onespace-supervisor.sh")
	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if !strings.Contains(string(data), `sh -c "exec $*"`) {
		t.Fatal("supervisor should exec the managed command so the pid file tracks the real process")
	}
}
