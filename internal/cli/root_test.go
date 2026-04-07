package cli

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"story-factory/internal/config"
	"story-factory/internal/output"
)

func TestRootCommand_FlagDefaults(t *testing.T) {
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&bytes.Buffer{}),
	}
	rootCmd := NewRootCommand(app)

	// Verify persistent flags exist with correct defaults
	dryRun, err := rootCmd.PersistentFlags().GetBool("dry-run")
	require.NoError(t, err)
	assert.False(t, dryRun)

	verbose, err := rootCmd.PersistentFlags().GetBool("verbose")
	require.NoError(t, err)
	assert.False(t, verbose)

	projectDir, err := rootCmd.PersistentFlags().GetString("project-dir")
	require.NoError(t, err)
	assert.Empty(t, projectDir)
}

func TestResolveProjectDir_ReturnsFlag(t *testing.T) {
	app := &App{ProjectDir: "/custom/path"}

	result, err := app.ResolveProjectDir()
	require.NoError(t, err)
	assert.Equal(t, "/custom/path", result)
}

func TestResolveProjectDir_FallbackToGetwd(t *testing.T) {
	app := &App{}

	result, err := app.ResolveProjectDir()
	require.NoError(t, err)

	wd, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, wd, result)
}

func TestResolveProjectDir_WhitespaceOnlyFallsBackToGetwd(t *testing.T) {
	app := &App{ProjectDir: "   \t  "}

	result, err := app.ResolveProjectDir()
	require.NoError(t, err)

	wd, err := os.Getwd()
	require.NoError(t, err)
	assert.Equal(t, wd, result)
}
