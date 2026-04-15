// Package git provides thin wrappers over the `git` CLI for the operations
// story-factory needs: inspecting branch state, creating a story branch,
// committing, and pushing. Phase 3 extends the package with worktree helpers.
//
// Each function shells out to the git binary rather than using go-git so
// behavior matches what a developer would see running the same command by
// hand. Functions accept an absolute `dir` (the repository working tree);
// tests can point at temp repositories created with [InitRepo].
package git

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// runGit runs `git <args...>` in dir and returns trimmed stdout. Stderr is
// captured into the returned error when the command fails.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// CurrentBranch returns the short name of the currently checked-out branch
// in the repository at dir. Returns an error if HEAD is detached.
func CurrentBranch(ctx context.Context, dir string) (string, error) {
	name, err := runGit(ctx, dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	if name == "HEAD" {
		return "", fmt.Errorf("%s: detached HEAD, no branch to use", dir)
	}
	return name, nil
}

// IsClean reports whether the working tree and index at dir have no pending
// changes. `git status --porcelain` with empty output is the definition of
// clean (tracked + untracked).
func IsClean(ctx context.Context, dir string) (bool, error) {
	out, err := runGit(ctx, dir, "status", "--porcelain")
	if err != nil {
		return false, err
	}
	return out == "", nil
}

// DefaultBranch returns the repository's default branch name. It first tries
// origin/HEAD, then falls back to main, then master. If none exist, returns
// an error so the caller can decide how to proceed.
func DefaultBranch(ctx context.Context, dir string) (string, error) {
	if out, err := runGit(ctx, dir, "symbolic-ref", "refs/remotes/origin/HEAD"); err == nil {
		// Output is like "refs/remotes/origin/main".
		parts := strings.Split(out, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1], nil
		}
	}
	for _, candidate := range []string{"main", "master"} {
		if _, err := runGit(ctx, dir, "show-ref", "--verify", "--quiet", "refs/heads/"+candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("%s: could not determine default branch (no origin/HEAD, no main, no master)", dir)
}

// CreateBranch creates and checks out branch name at dir. Returns an error
// if the branch already exists (use CheckoutBranch for idempotent checkout).
func CreateBranch(ctx context.Context, dir, name string) error {
	_, err := runGit(ctx, dir, "checkout", "-b", name)
	return err
}

// BranchExists reports whether a local branch with the given name exists in dir.
func BranchExists(ctx context.Context, dir, name string) (bool, error) {
	_, err := runGit(ctx, dir, "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	if err == nil {
		return true, nil
	}
	// git show-ref exits 1 when ref is missing — that's an expected outcome.
	var exitErr *exec.ExitError
	// Can't use errors.As directly because runGit wraps. Fall back to checking
	// message shape; cleaner to re-exec here.
	_ = exitErr
	cmd := exec.CommandContext(ctx, "git", "show-ref", "--verify", "--quiet", "refs/heads/"+name)
	cmd.Dir = dir
	if rerr := cmd.Run(); rerr != nil {
		if ee, ok := rerr.(*exec.ExitError); ok && ee.ExitCode() == 1 {
			return false, nil
		}
		return false, rerr
	}
	return true, nil
}

// AddAll stages all changes (tracked + untracked) in dir.
func AddAll(ctx context.Context, dir string) error {
	_, err := runGit(ctx, dir, "add", "-A")
	return err
}

// Commit creates a commit in dir with the given message. The message may
// contain newlines for a multi-line commit body.
func Commit(ctx context.Context, dir, message string) error {
	_, err := runGit(ctx, dir, "commit", "-m", message)
	return err
}

// PushUpstream pushes branch from dir to origin and sets upstream tracking.
func PushUpstream(ctx context.Context, dir, branch string) error {
	_, err := runGit(ctx, dir, "push", "-u", "origin", branch)
	return err
}

// HasRemote reports whether the named remote exists in dir (typically "origin").
func HasRemote(ctx context.Context, dir, name string) (bool, error) {
	out, err := runGit(ctx, dir, "remote")
	if err != nil {
		return false, err
	}
	for _, line := range strings.Split(out, "\n") {
		if strings.TrimSpace(line) == name {
			return true, nil
		}
	}
	return false, nil
}
