package gitx

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

type Service struct {
	Runner CommandRunner
}

type CommandRunner interface {
	Run(ctx context.Context, dir string, name string, args ...string) ([]byte, error)
}

func (s Service) Status(ctx context.Context, repoPath string) (Status, error) {
	var st Status
	st.RepoPath = repoPath

	if _, err := s.Runner.Run(ctx, repoPath, "git", "rev-parse", "--is-inside-work-tree"); err != nil {
		return Status{}, fmt.Errorf("not a git repo: %w", err)
	}

	branch, err := s.Runner.Run(ctx, repoPath, "git", "symbolic-ref", "--short", "HEAD")
	if err != nil {
		st.Detached = true
	} else {
		st.Branch = strings.TrimSpace(string(branch))
	}

	commit, err := s.Runner.Run(ctx, repoPath, "git", "rev-parse", "HEAD")
	if err == nil {
		st.Commit = strings.TrimSpace(string(commit))
	}

	porcelain, err := s.Runner.Run(ctx, repoPath, "git", "status", "--porcelain")
	if err == nil && len(strings.TrimSpace(string(porcelain))) > 0 {
		st.Dirty = true
	}

	tracking, err := s.Runner.Run(ctx, repoPath, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}")
	if err == nil {
		st.TrackingBranch = strings.TrimSpace(string(tracking))
	}

	if st.TrackingBranch != "" {
		counts, err := s.Runner.Run(ctx, repoPath, "git", "rev-list", "--left-right", "--count", "HEAD...@{u}")
		if err == nil {
			parts := strings.Fields(strings.TrimSpace(string(counts)))
			if len(parts) == 2 {
				st.Ahead, _ = strconv.Atoi(parts[0])
				st.Behind, _ = strconv.Atoi(parts[1])
			}
		}
	}

	remote, err := s.Runner.Run(ctx, repoPath, "git", "remote")
	if err == nil {
		remotes := strings.Fields(strings.TrimSpace(string(remote)))
		if len(remotes) > 0 {
			st.Remote = remotes[0]
		}
	}

	return st, nil
}

func (s Service) PullFastForwardOnly(ctx context.Context, repoPath string) (PullResult, error) {
	st, err := s.Status(ctx, repoPath)
	if err != nil {
		return PullResult{}, err
	}

	if st.Dirty {
		return PullResult{Status: st}, fmt.Errorf("refusing pull: working tree is dirty")
	}
	if st.Detached {
		return PullResult{Status: st}, fmt.Errorf("refusing pull: HEAD is detached")
	}
	if st.TrackingBranch == "" {
		return PullResult{Status: st}, fmt.Errorf("refusing pull: no tracking branch")
	}

	out, err := s.Runner.Run(ctx, repoPath, "git", "pull", "--ff-only")
	output := ""
	if out != nil {
		output = strings.TrimSpace(string(out))
	}
	if err != nil {
		return PullResult{Status: st, Output: output}, fmt.Errorf("git pull --ff-only failed: %w", err)
	}

	updated, err := s.Status(ctx, repoPath)
	if err != nil {
		return PullResult{Status: st, Output: output}, nil
	}
	return PullResult{Status: updated, Output: output}, nil
}
