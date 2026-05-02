package gitx

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

type osRunner struct{}

func (osRunner) Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func setupRepoWithRemote(t *testing.T) (local string, remote string) {
	t.Helper()
	dir := t.TempDir()
	remote = dir + "/remote.git"
	local = dir + "/local"

	runGit(t, dir, "init", "--bare", remote)
	runGit(t, dir, "init", local)
	runGit(t, local, "config", "user.email", "test@test.com")
	runGit(t, local, "config", "user.name", "Test")

	if err := exec.Command("sh", "-c", "echo hello > "+local+"/README.md").Run(); err != nil {
		t.Fatal(err)
	}
	runGit(t, local, "add", "README.md")
	runGit(t, local, "commit", "-m", "initial")
	runGit(t, local, "remote", "add", "origin", remote)
	runGit(t, local, "push", "-u", "origin", "main")
	return local, remote
}

func TestStatusReportsBranchCommitDirtyAndRemote(t *testing.T) {
	local, _ := setupRepoWithRemote(t)

	svc := Service{Runner: osRunner{}}
	st, err := svc.Status(context.Background(), local)
	if err != nil {
		t.Fatalf("Status error: %v", err)
	}
	if st.Branch != "main" {
		t.Fatalf("Branch = %q, want main", st.Branch)
	}
	if st.Commit == "" {
		t.Fatal("Commit is empty")
	}
	if st.Dirty {
		t.Fatal("should not be dirty")
	}
	if st.Remote != "origin" {
		t.Fatalf("Remote = %q, want origin", st.Remote)
	}
	if st.TrackingBranch == "" {
		t.Fatal("TrackingBranch is empty")
	}

	// Make it dirty
	if err := exec.Command("sh", "-c", "echo modified > "+local+"/README.md").Run(); err != nil {
		t.Fatal(err)
	}
	st, err = svc.Status(context.Background(), local)
	if err != nil {
		t.Fatalf("Status error: %v", err)
	}
	if !st.Dirty {
		t.Fatal("should be dirty after modification")
	}
}

func TestStatusIgnoresOnespaceRuntimeArtifacts(t *testing.T) {
	local, _ := setupRepoWithRemote(t)

	if err := exec.Command("sh", "-c", "mkdir -p "+local+"/.onespace/bin && echo app > "+local+"/.onespace/bin/app").Run(); err != nil {
		t.Fatal(err)
	}

	svc := Service{Runner: osRunner{}}
	st, err := svc.Status(context.Background(), local)
	if err != nil {
		t.Fatalf("Status error: %v", err)
	}
	if st.Dirty {
		t.Fatal("Onespace runtime artifacts should not make the repo dirty")
	}
}

func TestPullRefusesDirtyWorkingTree(t *testing.T) {
	local, _ := setupRepoWithRemote(t)

	if err := exec.Command("sh", "-c", "echo dirty > "+local+"/README.md").Run(); err != nil {
		t.Fatal(err)
	}

	svc := Service{Runner: osRunner{}}
	_, err := svc.PullFastForwardOnly(context.Background(), local)
	if err == nil {
		t.Fatal("expected error for dirty working tree")
	}
	if !strings.Contains(err.Error(), "dirty") {
		t.Fatalf("error should mention dirty, got: %v", err)
	}
}

func TestPullUsesFastForwardOnly(t *testing.T) {
	local, remote := setupRepoWithRemote(t)

	// Add a commit to the remote
	clone := t.TempDir() + "/clone"
	runGit(t, t.TempDir(), "clone", remote, clone)
	runGit(t, clone, "config", "user.email", "test@test.com")
	runGit(t, clone, "config", "user.name", "Test")
	if err := exec.Command("sh", "-c", "echo remote-change > "+clone+"/README.md").Run(); err != nil {
		t.Fatal(err)
	}
	runGit(t, clone, "add", "README.md")
	runGit(t, clone, "commit", "-m", "remote-update")
	runGit(t, clone, "push")

	svc := Service{Runner: osRunner{}}
	result, err := svc.PullFastForwardOnly(context.Background(), local)
	if err != nil {
		t.Fatalf("Pull error: %v", err)
	}
	if result.Status.Behind != 0 {
		t.Fatalf("should be up to date, behind = %d", result.Status.Behind)
	}
}
