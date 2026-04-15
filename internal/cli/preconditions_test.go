package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"story-factory/internal/output"
	"story-factory/internal/status"
)

// setupPreconditionEnv creates a temp directory with the specified precondition
// fixtures and changes the working directory to it. Returns a cleanup function.
func setupPreconditionEnv(t *testing.T, sprintStatus bool, bmadAgents bool) (string, func()) {
	t.Helper()

	dir := t.TempDir()

	if sprintStatus {
		statusDir := filepath.Dir(filepath.Join(dir, status.DefaultStatusPath))
		require.NoError(t, os.MkdirAll(statusDir, 0755))
		require.NoError(t, os.WriteFile(
			filepath.Join(dir, status.DefaultStatusPath),
			[]byte("development_status: {}\n"),
			0644,
		))
	}

	if bmadAgents {
		for _, name := range []string{"bmad-create-story", "bmad-dev-story", "bmad-code-review"} {
			agentDir := filepath.Join(dir, ".claude", "skills", name)
			require.NoError(t, os.MkdirAll(agentDir, 0755))
		}
	}

	origDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))

	return dir, func() {
		os.Chdir(origDir)
	}
}

func TestRunPreconditions_FailsOnMissingSprintStatus(t *testing.T) {
	// Set up agents but no sprint status (and bd may/may not be on PATH)
	_, cleanup := setupPreconditionEnv(t, false, true)
	defer cleanup()

	var buf bytes.Buffer
	app := &App{
		Printer: output.NewPrinterWithWriter(&buf),
	}

	err := app.RunPreconditions()
	require.Error(t, err)

	code, ok := IsExitError(err)
	assert.True(t, ok)
	assert.Equal(t, 2, code)

	// Output should contain a precondition failure message
	assert.Contains(t, buf.String(), "Precondition check failed")
}

func TestRunPreconditions_FailsOnMissingBMADAgents(t *testing.T) {
	_, cleanup := setupPreconditionEnv(t, true, false)
	defer cleanup()

	var buf bytes.Buffer
	app := &App{
		Printer: output.NewPrinterWithWriter(&buf),
	}

	err := app.RunPreconditions()
	require.Error(t, err)

	code, ok := IsExitError(err)
	assert.True(t, ok)
	assert.Equal(t, 2, code)

	assert.Contains(t, buf.String(), "Precondition check failed")
}

func TestRunPreconditions_ExitCode2OnFailure(t *testing.T) {
	// Empty dir — everything is missing
	_, cleanup := setupPreconditionEnv(t, false, false)
	defer cleanup()

	var buf bytes.Buffer
	app := &App{
		Printer: output.NewPrinterWithWriter(&buf),
	}

	err := app.RunPreconditions()
	require.Error(t, err)

	code, ok := IsExitError(err)
	assert.True(t, ok, "error should be an ExitError")
	assert.Equal(t, 2, code, "exit code should be 2 for precondition failures")
}

func TestRunPreconditions_SuccessWhenAllPresent_BmadMode(t *testing.T) {
	_, cleanup := setupPreconditionEnv(t, true, true)
	defer cleanup()

	var buf bytes.Buffer
	app := &App{
		Mode:    "bmad",
		Printer: output.NewPrinterWithWriter(&buf),
		// The real CheckAll requires git repo + gh CLI + clean tree in bmad
		// mode, which is environmental. Bypass with a stub so we only
		// exercise the RunPreconditions plumbing.
		CheckPreconditions: func(string) error { return nil },
	}

	err := app.RunPreconditions()
	assert.NoError(t, err, "stub precondition should succeed")
	assert.Empty(t, buf.String())
}

func TestRunPreconditions_BeadsMode_ChecksBdCLI(t *testing.T) {
	_, cleanup := setupPreconditionEnv(t, true, true)
	defer cleanup()

	var buf bytes.Buffer
	app := &App{
		Mode:    "beads",
		Printer: output.NewPrinterWithWriter(&buf),
	}

	err := app.RunPreconditions()
	// May fail if bd is not on PATH — that's acceptable in CI environments.
	if err != nil {
		code, ok := IsExitError(err)
		require.True(t, ok)
		assert.Equal(t, 2, code)
		assert.Contains(t, buf.String(), "bd CLI not found")
	} else {
		assert.Empty(t, buf.String())
	}
}
