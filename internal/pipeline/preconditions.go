package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"story-factory/internal/git"
	"story-factory/internal/status"
)

// CheckBdCLI verifies that the bd CLI is installed and on PATH.
// Returns a [PreconditionError] wrapping [ErrPreconditionFailed] if not found.
func CheckBdCLI() error {
	_, err := exec.LookPath("bd")
	if err != nil {
		return &PreconditionError{
			Check:  "bd-cli",
			Detail: "bd CLI not found on PATH — install Gastown Beads CLI",
		}
	}
	return nil
}

// CheckSprintStatus verifies that sprint-status.yaml exists at the expected
// project location. The path is resolved using [status.DefaultStatusPath].
func CheckSprintStatus(projectDir string) error {
	p := filepath.Join(projectDir, status.DefaultStatusPath)
	fi, err := os.Stat(p)
	if err != nil {
		return &PreconditionError{
			Check:  "sprint-status",
			Detail: "sprint-status.yaml not found at " + p,
		}
	}
	if !fi.Mode().IsRegular() {
		return &PreconditionError{
			Check:  "sprint-status",
			Detail: "sprint-status.yaml is not a regular file at " + p,
		}
	}
	return nil
}

// CheckBMADAgents verifies that BMAD skill directories required by the given
// mode are present in the project.
//
// All modes require bmad-create-story. BMAD mode additionally requires
// bmad-dev-story and bmad-code-review. Missing any required skill returns
// a [PreconditionError] wrapping [ErrPreconditionFailed].
func CheckBMADAgents(projectDir, mode string) error {
	required := []string{"bmad-create-story"}
	if mode == "bmad" {
		required = append(required, "bmad-dev-story", "bmad-code-review")
	}
	for _, name := range required {
		p := filepath.Join(projectDir, ".claude", "skills", name)
		fi, err := os.Stat(p)
		if err != nil {
			return &PreconditionError{
				Check:  "bmad-agents",
				Detail: fmt.Sprintf("BMAD skill %s/ not found — expected .claude/skills/%s/ in project", name, name),
			}
		}
		if !fi.IsDir() {
			return &PreconditionError{
				Check:  "bmad-agents",
				Detail: fmt.Sprintf("BMAD skill path .claude/skills/%s/ is not a directory", name),
			}
		}
	}
	return nil
}

// CheckGhCLI verifies that the gh CLI is installed and on PATH. Required
// by the open-pr step of bmad mode so a PR can be created after the commit.
func CheckGhCLI() error {
	_, err := exec.LookPath("gh")
	if err != nil {
		return &PreconditionError{
			Check:  "gh-cli",
			Detail: "gh CLI not found on PATH — install https://cli.github.com to open PRs",
		}
	}
	return nil
}

// CheckCleanWorkingTree verifies that the repository at projectDir is on a
// known branch and has no uncommitted changes. The commit-branch step
// requires this so it can branch off cleanly; a pre-existing dirty tree
// would end up in the first story's commit.
//
// If projectDir is not a git repository this check returns a PreconditionError
// rather than crashing — bmad mode requires git.
func CheckCleanWorkingTree(projectDir string) error {
	ctx := context.Background()
	if _, err := git.CurrentBranch(ctx, projectDir); err != nil {
		return &PreconditionError{
			Check:  "clean-tree",
			Detail: fmt.Sprintf("%s is not a git repository (or HEAD is detached): %v", projectDir, err),
		}
	}
	clean, err := git.IsClean(ctx, projectDir)
	if err != nil {
		return &PreconditionError{
			Check:  "clean-tree",
			Detail: fmt.Sprintf("failed to inspect working tree: %v", err),
		}
	}
	if !clean {
		return &PreconditionError{
			Check:  "clean-tree",
			Detail: "working tree has uncommitted changes — commit or stash before running bmad mode",
		}
	}
	return nil
}

// CheckAll runs the precondition checks appropriate for the given mode.
//
// All modes check sprint-status.yaml and the BMAD skill directories required
// by the mode. Beads mode additionally verifies that the bd CLI is on PATH.
// BMAD mode additionally verifies that the gh CLI is installed and the
// working tree is clean. Returns nil if all checks pass.
func CheckAll(projectDir, mode string) error {
	if mode == "" {
		mode = "bmad"
	}
	if err := CheckSprintStatus(projectDir); err != nil {
		return err
	}
	if err := CheckBMADAgents(projectDir, mode); err != nil {
		return err
	}
	switch mode {
	case "beads":
		if err := CheckBdCLI(); err != nil {
			return err
		}
	case "bmad":
		if err := CheckGhCLI(); err != nil {
			return err
		}
		if err := CheckCleanWorkingTree(projectDir); err != nil {
			return err
		}
	}
	return nil
}
