package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"story-factory/internal/claude"
	"story-factory/internal/config"
	"story-factory/internal/output"
	"story-factory/internal/status"
)

func TestRunCommand_RegisteredOnRoot(t *testing.T) {
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&bytes.Buffer{}),
	}

	rootCmd := NewRootCommand(app)

	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "run" {
			found = true
			break
		}
	}
	assert.True(t, found, "'run' subcommand should be registered on root command")
}

func TestRunCommand_RequiresExactlyOneArg(t *testing.T) {
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&bytes.Buffer{}),
	}

	rootCmd := NewRootCommand(app)

	// No args — should fail
	rootCmd.SetArgs([]string{"run"})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")

	// Too many args — should fail
	rootCmd2 := NewRootCommand(app)
	rootCmd2.SetArgs([]string{"run", "story-a", "story-b"})
	err = rootCmd2.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestRunCommand_DryRunPropagated(t *testing.T) {
	// Set up directory with sprint-status.yaml (backlog story so Run proceeds)
	dir := t.TempDir()
	key := "1-2-test-story"

	statusDir := filepath.Dir(filepath.Join(dir, status.DefaultStatusPath))
	require.NoError(t, os.MkdirAll(statusDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, status.DefaultStatusPath),
		[]byte(`story_location: "{project-root}/_bmad-output/implementation-artifacts"
development_status:
  `+key+`: backlog
`),
		0644,
	))

	var buf bytes.Buffer
	mock := &claude.MockExecutor{ExitCode: 0}
	app := &App{
		Config:             config.DefaultConfig(),
		Printer:            output.NewPrinterWithWriter(&buf),
		Executor:           mock,
		CheckPreconditions: func(string) error { return nil },
	}

	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"run", key, "--dry-run", "--project-dir", dir})
	err := rootCmd.Execute()
	require.NoError(t, err)

	// Dry-run means Claude should not have been called
	assert.Empty(t, mock.RecordedPrompts, "dry-run should prevent Claude invocation")
	assert.Contains(t, buf.String(), "dry-run")
}

func TestRunCommand_PreconditionFailureExitsCode2(t *testing.T) {
	// Empty temp dir with no preconditions met
	_, cleanup := setupPreconditionEnv(t, false, false)
	defer cleanup()

	var buf bytes.Buffer
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&buf),
	}

	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"run", "1-2-test-story"})

	err := rootCmd.Execute()
	require.Error(t, err)

	code, ok := IsExitError(err)
	require.True(t, ok, "should be ExitError")
	assert.Equal(t, 2, code, "precondition failure should exit with code 2")
}
