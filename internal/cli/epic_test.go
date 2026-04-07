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

func TestEpicCommand_RegisteredOnRoot(t *testing.T) {
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&bytes.Buffer{}),
	}

	rootCmd := NewRootCommand(app)

	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "epic" {
			found = true
			break
		}
	}
	assert.True(t, found, "'epic' subcommand should be registered on root command")
}

func TestEpicCommand_RequiresExactlyOneArg(t *testing.T) {
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&bytes.Buffer{}),
	}

	// No args — should fail
	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"epic"})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")

	// Too many args — should fail
	rootCmd2 := NewRootCommand(app)
	rootCmd2.SetArgs([]string{"epic", "1", "2"})
	err = rootCmd2.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepts 1 arg(s)")
}

func TestEpicCommand_NonIntegerArgReturnsError(t *testing.T) {
	app := &App{
		Config:             config.DefaultConfig(),
		Printer:            output.NewPrinterWithWriter(&bytes.Buffer{}),
		CheckPreconditions: func(string) error { return nil },
	}

	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"epic", "abc"})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid epic number")
}

func TestEpicCommand_ZeroArgReturnsError(t *testing.T) {
	app := &App{
		Config:             config.DefaultConfig(),
		Printer:            output.NewPrinterWithWriter(&bytes.Buffer{}),
		CheckPreconditions: func(string) error { return nil },
	}

	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"epic", "0"})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid epic number")
}

func TestEpicCommand_NegativeArgReturnsError(t *testing.T) {
	app := &App{
		Config:             config.DefaultConfig(),
		Printer:            output.NewPrinterWithWriter(&bytes.Buffer{}),
		CheckPreconditions: func(string) error { return nil },
	}

	// Cobra interprets "-1" as a flag; use "--" to force argument parsing
	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"epic", "--", "-1"})
	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid epic number")
}

func TestEpicCommand_PreconditionFailureExitsCode2(t *testing.T) {
	_, cleanup := setupPreconditionEnv(t, false, false)
	defer cleanup()

	var buf bytes.Buffer
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&buf),
	}

	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"epic", "1"})

	err := rootCmd.Execute()
	require.Error(t, err)

	code, ok := IsExitError(err)
	require.True(t, ok, "should be ExitError")
	assert.Equal(t, 2, code, "precondition failure should exit with code 2")
}

func TestEpicCommand_DryRunPropagated(t *testing.T) {
	dir := t.TempDir()

	statusDir := filepath.Dir(filepath.Join(dir, status.DefaultStatusPath))
	require.NoError(t, os.MkdirAll(statusDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, status.DefaultStatusPath),
		[]byte(`story_location: "{project-root}/_bmad-output/implementation-artifacts"
development_status:
  1-1-a: backlog
  1-2-b: backlog
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
	rootCmd.SetArgs([]string{"epic", "1", "--dry-run", "--project-dir", dir})
	err := rootCmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "dry-run")
}

func TestEpicCommand_ExitCode0OnAllSuccess(t *testing.T) {
	dir := t.TempDir()

	statusDir := filepath.Dir(filepath.Join(dir, status.DefaultStatusPath))
	require.NoError(t, os.MkdirAll(statusDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, status.DefaultStatusPath),
		[]byte(`story_location: "{project-root}/_bmad-output/implementation-artifacts"
development_status:
  1-1-a: backlog
  1-2-b: backlog
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
	rootCmd.SetArgs([]string{"epic", "1", "--dry-run", "--project-dir", dir})
	err := rootCmd.Execute()
	require.NoError(t, err, "all stories succeeded in dry-run, exit code should be 0")

	out := buf.String()
	assert.Contains(t, out, "BATCH COMPLETE")
}

func TestEpicCommand_ExitCode1OnPartialFailure(t *testing.T) {
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
	rootCmd.SetArgs([]string{"epic", "1", "--project-dir", dir})
	err := rootCmd.Execute()
	require.Error(t, err)

	code, ok := IsExitError(err)
	require.True(t, ok, "should be ExitError")
	assert.Equal(t, 1, code, "partial failure should exit with code 1")
}
