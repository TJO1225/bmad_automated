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

// mockStatusReader implements StatusReader for CLI tests.
type mockStatusReader struct {
	entries []status.Entry
	err     error
	locDir  string
	locErr  error
}

func (m *mockStatusReader) BacklogStories() ([]status.Entry, error) {
	return nil, nil
}

func (m *mockStatusReader) StoriesByStatus(s string) ([]status.Entry, error) {
	return nil, nil
}

func (m *mockStatusReader) StoriesForEpic(n int) ([]status.Entry, error) {
	return nil, nil
}

func (m *mockStatusReader) StoryByKey(key string) (*status.Entry, error) {
	if m.err != nil {
		return nil, m.err
	}
	for i := range m.entries {
		if m.entries[i].Key == key {
			return &m.entries[i], nil
		}
	}
	return nil, status.ErrStoryNotFound
}

func (m *mockStatusReader) ResolveStoryLocation(projectDir string) (string, error) {
	if m.locErr != nil {
		return "", m.locErr
	}
	return m.locDir, nil
}

func TestValidateStoryCommand_Registered(t *testing.T) {
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&bytes.Buffer{}),
	}
	rootCmd := NewRootCommand(app)

	// Verify validate-story is registered as a subcommand
	found := false
	for _, cmd := range rootCmd.Commands() {
		if cmd.Name() == "validate-story" {
			found = true
			assert.Equal(t, "validate-story <story-key>", cmd.Use)
			break
		}
	}
	assert.True(t, found, "validate-story command should be registered")
}

func TestValidateStoryCommand_RequiresExactlyOneArg(t *testing.T) {
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&bytes.Buffer{}),
	}
	rootCmd := NewRootCommand(app)

	// No args
	rootCmd.SetArgs([]string{"validate-story"})
	err := rootCmd.Execute()
	require.Error(t, err)
}

func TestValidateStoryCommand_PreconditionFailure(t *testing.T) {
	// Empty dir — preconditions will fail (no sprint-status.yaml, no BMAD agents)
	_, cleanup := setupPreconditionEnv(t, false, false)
	defer cleanup()

	var buf bytes.Buffer
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&buf),
		StatusReader: &mockStatusReader{
			entries: []status.Entry{
				{Key: "1-2-test", Status: status.StatusReadyForDev, Type: status.EntryTypeStory},
			},
		},
	}

	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"validate-story", "1-2-test"})
	err := rootCmd.Execute()
	require.Error(t, err)

	code, ok := IsExitError(err)
	assert.True(t, ok)
	assert.Equal(t, 2, code)
}

func TestValidateStoryCommand_StoryNotFound(t *testing.T) {
	// Set up a valid precondition environment
	_, cleanup := setupPreconditionEnv(t, true, true)
	defer cleanup()

	var buf bytes.Buffer
	app := &App{
		Config:  config.DefaultConfig(),
		Printer: output.NewPrinterWithWriter(&buf),
		StatusReader: &mockStatusReader{
			entries: []status.Entry{}, // empty — story won't be found
		},
		CheckPreconditions: func(string) error { return nil },
	}

	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"validate-story", "1-2-nonexistent"})
	err := rootCmd.Execute()
	require.Error(t, err)

	code, ok := IsExitError(err)
	require.True(t, ok)
	assert.Equal(t, 1, code)
	assert.Contains(t, buf.String(), "Story not found")
}

func TestValidateStoryCommand_DryRunPropagated(t *testing.T) {
	dir := t.TempDir()
	key := "1-2-test-story"

	statusDir := filepath.Dir(filepath.Join(dir, status.DefaultStatusPath))
	require.NoError(t, os.MkdirAll(statusDir, 0755))
	storyDir := filepath.Join(dir, "_bmad-output", "implementation-artifacts")
	require.NoError(t, os.MkdirAll(storyDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, status.DefaultStatusPath),
		[]byte(`story_location: "{project-root}/_bmad-output/implementation-artifacts"
development_status:
  `+key+`: ready-for-dev
`),
		0644,
	))
	require.NoError(t, os.WriteFile(
		filepath.Join(storyDir, key+".md"),
		[]byte("# Story 1.2: Test\n\n## Acceptance Criteria\n\n1. Given x When y Then z\n"),
		0644,
	))

	var buf bytes.Buffer
	app := &App{
		Config:             config.DefaultConfig(),
		Printer:            output.NewPrinterWithWriter(&buf),
		StatusReader:       status.NewReader(dir),
		CheckPreconditions: func(string) error { return nil },
	}

	rootCmd := NewRootCommand(app)
	rootCmd.SetArgs([]string{"validate-story", key, "--dry-run", "--project-dir", dir})
	err := rootCmd.Execute()
	require.NoError(t, err)

	assert.Contains(t, buf.String(), "dry-run")
}
