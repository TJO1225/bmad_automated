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

func TestCreateStoryCommand_Registered(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&buf),
	}
	rootCmd := NewRootCommand(app)

	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Use == "create-story <story-key>" {
			found = true
			break
		}
	}
	assert.True(t, found, "create-story should be a registered subcommand")
}

func TestCreateStoryCommand_MissingArgument(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&buf),
	}
	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"create-story"})
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Execute()
	require.Error(t, err)
}

func TestCreateStoryCommand_ExtraArguments(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&buf),
	}
	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"create-story", "key1", "key2"})
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Execute()
	require.Error(t, err)
}

func TestCreateStoryCommand_EmptyStoryKey(t *testing.T) {
	var buf bytes.Buffer
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&buf),
	}
	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"create-story", ""})
	rootCmd.SetOut(&buf)
	rootCmd.SetErr(&buf)

	err := rootCmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "empty")
}

func TestCreateStoryCommand_DryRunPropagated(t *testing.T) {
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
	app := &App{
		Config:             config.DefaultConfig(),
		Printer:            output.NewPrinterWithWriter(&buf),
		CheckPreconditions: func(string) error { return nil },
	}

	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"create-story", key, "--dry-run", "--project-dir", dir})
	err := rootCmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "dry-run")
}
