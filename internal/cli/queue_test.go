package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"story-factory/internal/config"
	"story-factory/internal/output"
	"story-factory/internal/status"
)

func TestQueueCommand_RegisteredOnRoot(t *testing.T) {
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&bytes.Buffer{}),
	}

	rootCmd := NewRootCommand(app)

	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "queue" {
			found = true
			break
		}
	}
	assert.True(t, found, "'queue' subcommand should be registered on root command")
}

func TestQueueCommand_AcceptsNoArgs(t *testing.T) {
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&bytes.Buffer{}),
	}

	// Extra args — should fail
	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"queue", "extra"})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown command")
}

func TestQueueCommand_PreconditionFailureExitsCode2(t *testing.T) {
	_, cleanup := setupPreconditionEnv(t, false, false)
	defer cleanup()

	var buf bytes.Buffer
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&buf),
	}

	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"queue"})

	err := rootCmd.Execute()
	require.Error(t, err)

	code, ok := IsExitError(err)
	require.True(t, ok, "should be ExitError")
	assert.Equal(t, 2, code, "precondition failure should exit with code 2")
}

func TestQueueCommand_DryRunPropagated(t *testing.T) {
	dir := t.TempDir()

	statusDir := filepath.Dir(filepath.Join(dir, status.DefaultStatusPath))
	require.NoError(t, os.MkdirAll(statusDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, status.DefaultStatusPath),
		[]byte(`story_location: "{project-root}/_bmad-output/implementation-artifacts"
development_status:
  1-1-a: backlog
  2-1-b: backlog
`),
		0644,
	))

	var buf bytes.Buffer
	app := &App{
		Config:             config.DefaultConfig(),
		Printer:            output.NewPrinterWithWriter(&buf),
		CheckPreconditions: func(string) error { return nil },
	}

	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"queue", "--dry-run", "--project-dir", dir})
	err := rootCmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "dry-run")
}

func TestQueueCommand_ExitCode0OnAllSuccess(t *testing.T) {
	dir := t.TempDir()

	statusDir := filepath.Dir(filepath.Join(dir, status.DefaultStatusPath))
	require.NoError(t, os.MkdirAll(statusDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, status.DefaultStatusPath),
		[]byte(`story_location: "{project-root}/_bmad-output/implementation-artifacts"
development_status:
  1-1-a: backlog
  2-1-b: backlog
`),
		0644,
	))

	var buf bytes.Buffer
	app := &App{
		Config:             config.DefaultConfig(),
		Printer:            output.NewPrinterWithWriter(&buf),
		CheckPreconditions: func(string) error { return nil },
	}

	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"queue", "--dry-run", "--project-dir", dir})
	err := rootCmd.Execute()
	require.NoError(t, err, "all stories succeeded in dry-run, exit code should be 0")

	out := buf.String()
	assert.Contains(t, out, "QUEUE COMPLETE")
}

func TestQueueCommand_ExitCode1OnPartialFailure(t *testing.T) {
	dir := t.TempDir()

	statusDir := filepath.Dir(filepath.Join(dir, status.DefaultStatusPath))
	require.NoError(t, os.MkdirAll(statusDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, status.DefaultStatusPath),
		[]byte(`story_location: "{project-root}/_bmad-output/implementation-artifacts"
development_status:
  1-1-a: backlog
`),
		0644,
	))

	cfg := config.DefaultConfig()
	cfg.Claude.BinaryPath = "/nonexistent/story-factory-test-binary"

	var buf bytes.Buffer
	app := &App{
		Config:             cfg,
		Printer:            output.NewPrinterWithWriter(&buf),
		CheckPreconditions: func(string) error { return nil },
	}

	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"queue", "--project-dir", dir})
	err := rootCmd.Execute()
	require.Error(t, err)

	code, ok := IsExitError(err)
	require.True(t, ok, "should be ExitError")
	assert.Equal(t, 1, code, "partial failure should exit with code 1")
}
