package beads

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Executor runs the bd CLI to create beads.
type Executor interface {
	// Create invokes `bd create` with the story key, title, and acceptance criteria.
	// Returns the bead ID on success. Uses context for timeout/cancellation.
	//
	// If bdOut is non-nil, a copy of bd's stdout and stderr is written to bdOut as
	// the subprocess runs (for verbose CLI output).
	Create(ctx context.Context, key, title, acs string, bdOut io.Writer) (beadID string, err error)
}

// DefaultExecutor implements [Executor] by shelling out to the bd CLI.
type DefaultExecutor struct {
	// BinaryPath is the path to the bd binary. Defaults to "bd" if empty.
	BinaryPath string
}

// NewExecutor creates a new [DefaultExecutor] with the bd binary on PATH.
func NewExecutor() *DefaultExecutor {
	return &DefaultExecutor{BinaryPath: "bd"}
}

// Create invokes `bd create "<key>: <title>" --notes "<acs>"` and parses the
// bead ID from stdout. A 30-second timeout is applied if the context has no
// deadline set.
func (e *DefaultExecutor) Create(ctx context.Context, key, title, acs string, bdOut io.Writer) (string, error) {
	binary := e.BinaryPath
	if binary == "" {
		binary = "bd"
	}

	// Apply 30s timeout if context has no deadline
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
	}

	label := fmt.Sprintf("%s: %s", key, title)
	cmd := exec.CommandContext(ctx, binary, "create", label, "--notes", acs)

	var stdout, stderr bytes.Buffer
	if bdOut != nil {
		cmd.Stdout = io.MultiWriter(&stdout, bdOut)
		cmd.Stderr = io.MultiWriter(&stderr, bdOut)
	} else {
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
	}

	if err := cmd.Run(); err != nil {
		stderrMsg := strings.TrimSpace(stderr.String())
		if stderrMsg != "" {
			return "", fmt.Errorf("bd create failed: %s", stderrMsg)
		}
		return "", fmt.Errorf("bd create failed: %w", err)
	}

	beadID := ParseBeadID(stdout.String())
	if beadID == "" {
		return "", fmt.Errorf("bd create returned no bead ID")
	}

	return beadID, nil
}

// ParseBeadID extracts the bead ID from bd create stdout.
// It returns the first non-empty line, trimmed of whitespace.
func ParseBeadID(stdout string) string {
	for _, line := range strings.Split(stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// AppendTrackingComment appends a `<!-- bead:{beadID} -->` HTML comment to the
// end of the story file. It checks for an existing tracking comment to avoid
// duplicates.
func AppendTrackingComment(storyPath, beadID string) error {
	content, err := os.ReadFile(storyPath)
	if err != nil {
		return fmt.Errorf("read story file: %w", err)
	}

	comment := fmt.Sprintf("<!-- bead:%s -->", beadID)

	// Check for existing tracking comment
	if strings.Contains(string(content), comment) {
		return nil // already tagged
	}

	f, err := os.OpenFile(storyPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open story file for append: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString("\n" + comment + "\n"); err != nil {
		return fmt.Errorf("write tracking comment: %w", err)
	}

	return nil
}
