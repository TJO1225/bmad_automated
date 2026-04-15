package git

import (
	"context"
	"fmt"
	"strings"
)

// WorktreeInfo describes a single `git worktree` entry as parsed from
// `git worktree list --porcelain`.
type WorktreeInfo struct {
	// Path is the absolute worktree directory.
	Path string
	// Branch is the short branch name (e.g. "story/1-1-foo"). Empty if the
	// worktree is detached.
	Branch string
	// Head is the commit SHA currently checked out in the worktree.
	Head string
	// Detached reports whether the worktree is in detached-HEAD state.
	Detached bool
	// Bare reports whether the entry describes a bare repository (primary
	// linked worktree uses the parent repository's config; bare is a
	// top-level flag on the list output).
	Bare bool
}

// AddWorktree creates a new worktree at path, checked out on a fresh branch
// named `branch` that points at `base`. Equivalent to:
//
//	git -C repoRoot worktree add -b <branch> <path> <base>
//
// The branch must not already exist locally; callers are responsible for
// deleting any leftover branch from a prior aborted run (via DeleteBranch)
// before retrying.
func AddWorktree(ctx context.Context, repoRoot, path, branch, base string) error {
	_, err := runGit(ctx, repoRoot, "worktree", "add", "-b", branch, path, base)
	return err
}

// RemoveWorktree removes the worktree at path. Equivalent to
// `git -C repoRoot worktree remove <path>`. Fails if the worktree has
// uncommitted changes; callers should force-remove only after verifying
// it's safe.
func RemoveWorktree(ctx context.Context, repoRoot, path string) error {
	_, err := runGit(ctx, repoRoot, "worktree", "remove", path)
	return err
}

// ListWorktrees returns all worktrees tracked by the repository at repoRoot.
// The first entry is always the primary worktree (repoRoot itself). The
// parsed output comes from `git worktree list --porcelain`.
func ListWorktrees(ctx context.Context, repoRoot string) ([]WorktreeInfo, error) {
	out, err := runGit(ctx, repoRoot, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var worktrees []WorktreeInfo
	var current WorktreeInfo
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			if current.Path != "" {
				worktrees = append(worktrees, current)
				current = WorktreeInfo{}
			}
			continue
		}
		switch {
		case strings.HasPrefix(line, "worktree "):
			current.Path = strings.TrimPrefix(line, "worktree ")
		case strings.HasPrefix(line, "HEAD "):
			current.Head = strings.TrimPrefix(line, "HEAD ")
		case strings.HasPrefix(line, "branch "):
			// Format: "branch refs/heads/main" — keep just the short name.
			ref := strings.TrimPrefix(line, "branch ")
			current.Branch = strings.TrimPrefix(ref, "refs/heads/")
		case line == "detached":
			current.Detached = true
		case line == "bare":
			current.Bare = true
		}
	}
	// Flush the last entry if the output didn't end with a blank line.
	if current.Path != "" {
		worktrees = append(worktrees, current)
	}
	return worktrees, nil
}

// IsBranchMerged reports whether `branch` has been merged into `into` in the
// repository at repoRoot. Used by `story-factory cleanup` to decide which
// worktrees are safe to remove.
//
// Implementation: `git -C repoRoot merge-base --is-ancestor <branch> <into>`
// exits 0 when branch is an ancestor of into (i.e. merged).
func IsBranchMerged(ctx context.Context, repoRoot, branch, into string) (bool, error) {
	_, err := runGit(ctx, repoRoot, "merge-base", "--is-ancestor", branch, into)
	if err == nil {
		return true, nil
	}
	// merge-base --is-ancestor exits 1 when not an ancestor; anything else is an error.
	if strings.Contains(err.Error(), "exit status 1") {
		return false, nil
	}
	return false, err
}

// DeleteBranch deletes the branch from repoRoot. `force` uses -D (allows
// deletion even if not merged); otherwise -d (safe delete).
func DeleteBranch(ctx context.Context, repoRoot, branch string, force bool) error {
	flag := "-d"
	if force {
		flag = "-D"
	}
	_, err := runGit(ctx, repoRoot, "branch", flag, branch)
	return err
}

// CurrentBranchName returns the short branch name at HEAD, or "" if HEAD is
// detached. Unlike [CurrentBranch], detached HEAD is not an error — callers
// that want to act on the fact (e.g. create a new branch) can distinguish
// the two states by checking for an empty return.
func CurrentBranchName(ctx context.Context, dir string) (string, error) {
	out, err := runGit(ctx, dir, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil {
		return out, nil
	}
	// symbolic-ref exits 1 on detached HEAD; confirm by checking HEAD resolves.
	if _, rerr := runGit(ctx, dir, "rev-parse", "--verify", "HEAD"); rerr == nil {
		return "", nil
	}
	return "", fmt.Errorf("CurrentBranchName: %w", err)
}
