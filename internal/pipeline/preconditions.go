package pipeline

import (
	"os"
	"os/exec"
	"path/filepath"

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

// CheckBMADAgents verifies that BMAD agent files are present in the project.
// It checks for the .claude/skills/bmad-create-story/ directory as the minimum
// signal that BMAD is installed.
func CheckBMADAgents(projectDir string) error {
	p := filepath.Join(projectDir, ".claude", "skills", "bmad-create-story")
	fi, err := os.Stat(p)
	if err != nil {
		return &PreconditionError{
			Check:  "bmad-agents",
			Detail: "BMAD agent files not found — expected .claude/skills/bmad-create-story/ in project",
		}
	}
	if !fi.IsDir() {
		return &PreconditionError{
			Check:  "bmad-agents",
			Detail: "BMAD skill path is not a directory — expected .claude/skills/bmad-create-story/ in project",
		}
	}
	return nil
}

// CheckAll runs all precondition checks in order and returns the first failure.
// Check order: bd CLI → sprint-status.yaml → BMAD agents.
// Returns nil if all checks pass.
func CheckAll(projectDir string) error {
	if err := CheckBdCLI(); err != nil {
		return err
	}
	if err := CheckSprintStatus(projectDir); err != nil {
		return err
	}
	if err := CheckBMADAgents(projectDir); err != nil {
		return err
	}
	return nil
}
