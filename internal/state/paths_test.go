package state

import "testing"

func TestWorkspaceStatePathsStayUnderWorkspace(t *testing.T) {
	p := Paths{WorkspaceRoot: "/data/workspaces/my-workspace"}

	tests := []struct {
		method string
		got    string
		want   string
	}{
		{"StateDir", p.StateDir(), "/data/workspaces/my-workspace/state"},
		{"JobsLogDir", p.JobsLogDir(), "/data/workspaces/my-workspace/state/logs/jobs"},
		{"ServicesLogDir", p.ServicesLogDir(), "/data/workspaces/my-workspace/state/logs/services"},
		{"GeneratedDir", p.GeneratedDir(), "/data/workspaces/my-workspace/generated"},
	}

	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %q, want %q", tt.method, tt.got, tt.want)
		}
	}
}
