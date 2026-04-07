package pipeline

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"story-factory/internal/beads"
	"story-factory/internal/claude"
	"story-factory/internal/config"
	"story-factory/internal/status"
)

// testValidationExecutor wraps the claude.Executor interface and optionally
// modifies the story file during execution to simulate BMAD applying suggestions.
type testValidationExecutor struct {
	filePath     string
	modifyOnCall []bool // per-call: true = touch file, false = leave unchanged
	callCount    int
	exitCodes    []int // per-call exit codes (default 0)
	failOnCall   []error
}

func (m *testValidationExecutor) Execute(ctx context.Context, prompt string) (<-chan claude.Event, error) {
	ch := make(chan claude.Event)
	close(ch)
	return ch, nil
}

func (m *testValidationExecutor) ExecuteWithResult(ctx context.Context, prompt string, handler claude.EventHandler) (int, error) {
	idx := m.callCount
	m.callCount++

	if idx < len(m.failOnCall) && m.failOnCall[idx] != nil {
		return 1, m.failOnCall[idx]
	}

	if idx < len(m.modifyOnCall) && m.modifyOnCall[idx] {
		// Write new content AND explicitly advance mtime to ensure detection.
		// Filesystem mtime resolution varies; explicit Chtimes guarantees a
		// detectable change regardless of how fast the test runs.
		err := os.WriteFile(m.filePath, []byte(fmt.Sprintf("modified-%d", idx)), 0644)
		if err != nil {
			return 1, err
		}
		future := time.Now().Add(time.Duration(idx+1) * time.Second)
		_ = os.Chtimes(m.filePath, future, future)
	}

	exitCode := 0
	if idx < len(m.exitCodes) {
		exitCode = m.exitCodes[idx]
	}
	return exitCode, nil
}

// testConfig returns a minimal config with the create-story prompt template.
func testConfig() *config.Config {
	cfg := config.DefaultConfig()
	return cfg
}

// setupStoryFile creates a story markdown file (caller must already have
// written sprint-status.yaml with story_location, e.g. via writeMinimalSprintStatus).
func setupStoryFile(t *testing.T, dir string, key string) string {
	t.Helper()
	storyDir := filepath.Join(dir, "_bmad-output", "implementation-artifacts")
	require.NoError(t, os.MkdirAll(storyDir, 0755))
	storyPath := filepath.Join(storyDir, key+".md")
	require.NoError(t, os.WriteFile(storyPath, []byte("# Story\n"), 0644))
	return storyPath
}

func TestStepValidate(t *testing.T) {
	tests := []struct {
		name            string
		modifyOnCall    []bool
		exitCodes       []int
		failOnCall      []error
		createFile      bool
		wantSuccess     bool
		wantReason      string
		wantLoops       int
		wantErr         bool
		wantErrContains string
	}{
		{
			name:         "converges on first loop (mtime unchanged)",
			modifyOnCall: []bool{false},
			createFile:   true,
			wantSuccess:  true,
			wantLoops:    1,
		},
		{
			name:         "converges on 2nd loop (mtime changes once then stable)",
			modifyOnCall: []bool{true, false},
			createFile:   true,
			wantSuccess:  true,
			wantLoops:    2,
		},
		{
			name:         "converges on 3rd loop",
			modifyOnCall: []bool{true, true, false},
			createFile:   true,
			wantSuccess:  true,
			wantLoops:    3,
		},
		{
			name:         "exhaustion after 3 loops (mtime always changes)",
			modifyOnCall: []bool{true, true, true},
			createFile:   true,
			wantSuccess:  false,
			wantReason:   "needs-review",
			wantLoops:    3,
		},
		{
			name:            "claude subprocess failure returns error",
			failOnCall:      []error{fmt.Errorf("connection refused")},
			createFile:      true,
			wantErr:         true,
			wantErrContains: "claude execution failed",
		},
		{
			name:         "claude non-zero exit is operational failure",
			exitCodes:       []int{1},
			createFile:      true,
			wantSuccess:     false,
			wantReason:      "validate story 1-2-test-story: claude exited with code 1",
			wantLoops:       1,
		},
		{
			name:            "missing story file returns error",
			createFile:      false,
			wantErr:         true,
			wantErrContains: "cannot stat story file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			key := "1-2-test-story"

			writeMinimalSprintStatus(t, dir, key)

			var storyPath string
			if tt.createFile {
				storyPath = setupStoryFile(t, dir, key)
			} else {
				storyPath = filepath.Join(dir, "_bmad-output", "implementation-artifacts", key+".md")
			}

			executor := &testValidationExecutor{
				filePath:     storyPath,
				modifyOnCall: tt.modifyOnCall,
				exitCodes:    tt.exitCodes,
				failOnCall:   tt.failOnCall,
			}

			cfg := testConfig()
			p := NewPipeline(executor, cfg, dir,
				WithStatus(status.NewReader(dir)),
			)

			result, err := p.StepValidate(context.Background(), key)

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContains)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, stepNameValidate, result.Name)
			assert.Equal(t, tt.wantSuccess, result.Success)
			assert.Equal(t, tt.wantReason, result.Reason)
			assert.Equal(t, tt.wantLoops, result.ValidationLoops)
			assert.True(t, result.Duration > 0, "duration should be positive")
		})
	}
}

func TestStepValidate_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	key := "1-2-test-story"
	writeMinimalSprintStatus(t, dir, key)
	storyPath := setupStoryFile(t, dir, key)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	executor := &testValidationExecutor{
		filePath: storyPath,
		failOnCall: []error{
			fmt.Errorf("context canceled"),
		},
	}

	cfg := testConfig()
	p := NewPipeline(executor, cfg, dir,
		WithStatus(status.NewReader(dir)),
	)

	result, err := p.StepValidate(ctx, key)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Reason, "canceled")
}

// --- StepCreate tests ---

// writeMinimalSprintStatus writes sprint-status.yaml so [status.Reader.ResolveStoryLocation] works.
func writeMinimalSprintStatus(t *testing.T, dir, key string) {
	t.Helper()
	statusDir := filepath.Dir(filepath.Join(dir, status.DefaultStatusPath))
	require.NoError(t, os.MkdirAll(statusDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, status.DefaultStatusPath),
		[]byte(sprintStatusFixture(key, "ready-for-dev")),
		0644,
	))
}

// sprintStatusFixture returns sprint-status.yaml content with the given key/status.
func sprintStatusFixture(key, stat string) string {
	return `story_location: "{project-root}/_bmad-output/implementation-artifacts"
development_status:
  ` + key + `: ` + stat + "\n"
}

// setupStepCreateDir creates a temp dir with sprint-status.yaml and optionally
// the story file.
func setupStepCreateDir(t *testing.T, key, yamlStatus string, createFile bool) string {
	t.Helper()
	dir := t.TempDir()

	statusDir := filepath.Dir(filepath.Join(dir, status.DefaultStatusPath))
	require.NoError(t, os.MkdirAll(statusDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, status.DefaultStatusPath),
		[]byte(sprintStatusFixture(key, yamlStatus)),
		0644,
	))

	if createFile {
		storyDir := filepath.Join(dir, "_bmad-output", "implementation-artifacts")
		require.NoError(t, os.WriteFile(
			filepath.Join(storyDir, key+".md"),
			[]byte("# Story "+key+"\n"),
			0644,
		))
	}

	return dir
}

type mockPrinter struct {
	texts []string
}

func (m *mockPrinter) Text(message string)                          { m.texts = append(m.texts, message) }
func (m *mockPrinter) StepStart(step, total int, name string)       {}
func (m *mockPrinter) StepEnd(duration time.Duration, success bool) {}

func TestStepCreate_Success(t *testing.T) {
	key := "1-2-database-schema"
	dir := setupStepCreateDir(t, key, "ready-for-dev", true)
	printer := &mockPrinter{}

	mock := &claude.MockExecutor{ExitCode: 0}
	p := NewPipeline(mock, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
		WithPrinter(printer),
	)

	result, err := p.StepCreate(context.Background(), key)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "create", result.Name)
	assert.True(t, result.Duration > 0)
	assert.Len(t, mock.RecordedPrompts, 1)
	assert.Contains(t, mock.RecordedPrompts[0], key)
}

func TestStepCreate_FileNotCreated(t *testing.T) {
	key := "1-2-database-schema"
	dir := setupStepCreateDir(t, key, "ready-for-dev", false)

	mock := &claude.MockExecutor{ExitCode: 0}
	p := NewPipeline(mock, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
	)

	result, err := p.StepCreate(context.Background(), key)

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Reason, "not created")
	assert.Equal(t, "create", result.Name)
}

func TestStepCreate_StatusNotUpdated(t *testing.T) {
	key := "1-2-database-schema"
	dir := setupStepCreateDir(t, key, "backlog", true)

	mock := &claude.MockExecutor{ExitCode: 0}
	p := NewPipeline(mock, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
	)

	result, err := p.StepCreate(context.Background(), key)

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Reason, "sprint status not updated")
}

func TestStepCreate_ClaudeNonZeroExit(t *testing.T) {
	key := "1-2-database-schema"
	dir := setupStepCreateDir(t, key, "ready-for-dev", true)

	mock := &claude.MockExecutor{ExitCode: 1}
	p := NewPipeline(mock, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
	)

	result, err := p.StepCreate(context.Background(), key)

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Reason, "claude exited with code 1")
}

func TestStepCreate_ClaudeError(t *testing.T) {
	key := "1-2-database-schema"
	dir := setupStepCreateDir(t, key, "ready-for-dev", true)

	mock := &claude.MockExecutor{Error: errors.New("binary not found")}
	p := NewPipeline(mock, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
	)

	_, err := p.StepCreate(context.Background(), key)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "binary not found")
}

func TestStepCreate_ContextTimeout(t *testing.T) {
	key := "1-2-database-schema"
	dir := setupStepCreateDir(t, key, "ready-for-dev", true)

	mock := &claude.MockExecutor{Error: context.DeadlineExceeded}
	p := NewPipeline(mock, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
	)

	result, err := p.StepCreate(context.Background(), key)

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Reason, "timed out")
}

func TestStepCreate_ContextCanceled(t *testing.T) {
	key := "1-2-database-schema"
	dir := setupStepCreateDir(t, key, "ready-for-dev", true)

	mock := &claude.MockExecutor{Error: context.Canceled}
	p := NewPipeline(mock, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
	)

	result, err := p.StepCreate(context.Background(), key)

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Reason, "canceled")
}

func TestStepCreate_NilStatusReader(t *testing.T) {
	key := "1-2-database-schema"
	dir := setupStepCreateDir(t, key, "ready-for-dev", true)

	mock := &claude.MockExecutor{ExitCode: 0}
	p := NewPipeline(mock, config.DefaultConfig(), dir)

	_, err := p.StepCreate(context.Background(), key)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "status reader")
}

func TestStepCreate_StoryKeyNotInSprintStatus(t *testing.T) {
	key := "1-2-database-schema"
	dir := t.TempDir()
	statusDir := filepath.Dir(filepath.Join(dir, status.DefaultStatusPath))
	require.NoError(t, os.MkdirAll(statusDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, status.DefaultStatusPath),
		[]byte(sprintStatusFixture("9-9-other-story", "ready-for-dev")),
		0644,
	))
	storyDir := filepath.Join(dir, "_bmad-output", "implementation-artifacts")
	require.NoError(t, os.WriteFile(
		filepath.Join(storyDir, key+".md"),
		[]byte("# Story\n"),
		0644,
	))

	mock := &claude.MockExecutor{ExitCode: 0}
	p := NewPipeline(mock, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
	)

	result, err := p.StepCreate(context.Background(), key)

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Reason, "not in sprint-status")
}

func TestStepCreate_DryRun(t *testing.T) {
	key := "1-2-database-schema"
	dir := setupStepCreateDir(t, key, "backlog", false)
	printer := &mockPrinter{}

	mock := &claude.MockExecutor{ExitCode: 0}
	p := NewPipeline(mock, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
		WithPrinter(printer),
		WithDryRun(true),
	)

	result, err := p.StepCreate(context.Background(), key)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Reason, "dry-run")
	assert.Empty(t, mock.RecordedPrompts)
	require.Len(t, printer.texts, 1)
	assert.Contains(t, printer.texts[0], "dry-run")
}

func TestStepCreate_PromptExpansion(t *testing.T) {
	key := "3-1-user-auth"
	dir := setupStepCreateDir(t, key, "ready-for-dev", true)

	mock := &claude.MockExecutor{ExitCode: 0}
	p := NewPipeline(mock, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
	)

	_, err := p.StepCreate(context.Background(), key)
	require.NoError(t, err)

	require.Len(t, mock.RecordedPrompts, 1)
	assert.Contains(t, mock.RecordedPrompts[0], key)
}

func TestStepCreate_VerboseForwardsEvents(t *testing.T) {
	key := "1-2-database-schema"
	dir := setupStepCreateDir(t, key, "ready-for-dev", true)
	printer := &mockPrinter{}

	mock := &claude.MockExecutor{
		ExitCode: 0,
		Events: []claude.Event{
			{Type: claude.EventTypeAssistant, Text: "Working on it..."},
			{Type: claude.EventTypeAssistant, Text: "Done!"},
		},
	}
	p := NewPipeline(mock, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
		WithPrinter(printer),
		WithVerbose(true),
	)

	result, err := p.StepCreate(context.Background(), key)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, []string{"Working on it...", "Done!"}, printer.texts)
}

// --- StepSync tests ---

// writeTestStory creates a valid story file at the given path.
func writeTestStory(t *testing.T, path string) {
	t.Helper()
	dir := filepath.Dir(path)
	require.NoError(t, os.MkdirAll(dir, 0755))
	content := `# Story 1.2: Database Schema

## Acceptance Criteria

1. Given a new installation
   When migrations run
   Then all tables are created

## Tasks / Subtasks

- [ ] Task 1: Create migration files
`
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func TestStepSync_Success(t *testing.T) {
	dir := t.TempDir()
	storyPath := filepath.Join(dir, "1-2-database-schema.md")
	writeTestStory(t, storyPath)

	mock := &beads.MockExecutor{BeadID: "bd-abc123"}

	result, err := StepSync(context.Background(), mock, storyPath, "1-2-database-schema")

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "sync", result.Name)
	assert.Equal(t, "bd-abc123", result.BeadID)
	assert.True(t, result.Duration > 0)

	// Verify mock was called correctly
	require.Len(t, mock.Calls, 1)
	assert.Equal(t, "1-2-database-schema", mock.Calls[0].Key)
	assert.Equal(t, "Database Schema", mock.Calls[0].Title)
	assert.NotEmpty(t, mock.Calls[0].ACs)

	// Verify tracking comment was appended
	content, err := os.ReadFile(storyPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "<!-- bead:bd-abc123 -->")
}

func TestStepSync_BdCreateFailure(t *testing.T) {
	dir := t.TempDir()
	storyPath := filepath.Join(dir, "1-2-database-schema.md")
	writeTestStory(t, storyPath)

	mock := &beads.MockExecutor{Err: fmt.Errorf("connection refused")}

	result, err := StepSync(context.Background(), mock, storyPath, "1-2-database-schema")

	require.NoError(t, err) // operational failure, not infra
	assert.False(t, result.Success)
	assert.Equal(t, "sync", result.Name)
	assert.Contains(t, result.Reason, "bd create")
	assert.Contains(t, result.Reason, "connection refused")

	// Verify story file was NOT modified with tracking comment
	content, err := os.ReadFile(storyPath)
	require.NoError(t, err)
	assert.NotContains(t, string(content), "<!-- bead:")
}

func TestStepSync_MalformedStory_NoTitle(t *testing.T) {
	dir := t.TempDir()
	storyPath := filepath.Join(dir, "bad-story.md")
	require.NoError(t, os.WriteFile(storyPath, []byte("## No title heading\n\nSome content\n"), 0644))

	mock := &beads.MockExecutor{BeadID: "bd-should-not-reach"}

	result, err := StepSync(context.Background(), mock, storyPath, "bad-story")

	require.NoError(t, err) // operational failure
	assert.False(t, result.Success)
	assert.Contains(t, result.Reason, "extract title")
	assert.Empty(t, mock.Calls) // bd create should not be called
}

func TestStepSync_MalformedStory_NoACs(t *testing.T) {
	dir := t.TempDir()
	storyPath := filepath.Join(dir, "no-acs.md")
	content := "# Story 1.2: Has Title\n\n## Tasks\n\n- [ ] Do something\n"
	require.NoError(t, os.WriteFile(storyPath, []byte(content), 0644))

	mock := &beads.MockExecutor{BeadID: "bd-should-not-reach"}

	result, err := StepSync(context.Background(), mock, storyPath, "no-acs")

	require.NoError(t, err) // operational failure
	assert.False(t, result.Success)
	assert.Contains(t, result.Reason, "extract ACs")
	assert.Empty(t, mock.Calls)
}

func TestStepSync_FileNotFound(t *testing.T) {
	mock := &beads.MockExecutor{BeadID: "bd-irrelevant"}

	result, err := StepSync(context.Background(), mock, "/no/such/file.md", "missing")

	require.Error(t, err) // infrastructure failure
	assert.Contains(t, err.Error(), "read story file")
	assert.Equal(t, "sync", result.Name)
	assert.Empty(t, mock.Calls)
}

func TestStepValidate_DryRun(t *testing.T) {
	dir := t.TempDir()
	key := "1-2-test-story"
	writeMinimalSprintStatus(t, dir, key)
	printer := &mockPrinter{}

	executor := &testValidationExecutor{}

	cfg := testConfig()
	p := NewPipeline(executor, cfg, dir,
		WithStatus(status.NewReader(dir)),
		WithPrinter(printer),
		WithDryRun(true),
	)

	result, err := p.StepValidate(context.Background(), key)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Reason, "dry-run")
	assert.Equal(t, stepNameValidate, result.Name)
	assert.Equal(t, 0, executor.callCount, "should not invoke Claude in dry-run")
	require.Len(t, printer.texts, 1)
	assert.Contains(t, printer.texts[0], "dry-run")
}

func TestStepSync_DryRun(t *testing.T) {
	dir := t.TempDir()
	key := "1-2-test-story"
	writeMinimalSprintStatus(t, dir, key)
	printer := &mockPrinter{}

	beadsMock := &beads.MockExecutor{BeadID: "bd-should-not-reach"}

	cfg := testConfig()
	p := NewPipeline(&claude.MockExecutor{}, cfg, dir,
		WithStatus(status.NewReader(dir)),
		WithPrinter(printer),
		WithBeads(beadsMock),
		WithDryRun(true),
	)

	result, err := p.stepSync(context.Background(), key)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Reason, "dry-run")
	assert.Equal(t, stepNameSync, result.Name)
	assert.Empty(t, beadsMock.Calls, "should not invoke bd in dry-run")
	require.Len(t, printer.texts, 1)
	assert.Contains(t, printer.texts[0], "dry-run")
}

func TestStepSync_Verbose(t *testing.T) {
	dir := t.TempDir()
	key := "1-2-database-schema"
	writeMinimalSprintStatus(t, dir, key)
	storyPath := setupStoryFile(t, dir, key)
	writeTestStory(t, storyPath)
	printer := &mockPrinter{}

	beadsMock := &beads.MockExecutor{BeadID: "bd-verbose-123"}

	cfg := testConfig()
	p := NewPipeline(&claude.MockExecutor{}, cfg, dir,
		WithStatus(status.NewReader(dir)),
		WithPrinter(printer),
		WithBeads(beadsMock),
		WithVerbose(true),
	)

	result, err := p.stepSync(context.Background(), key)

	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, "bd-verbose-123", result.BeadID)

	// Verbose: real bd stdout (mock) forwarded to printer line-by-line
	require.NotEmpty(t, printer.texts)
	assert.Contains(t, printer.texts[0], "mock bd stdout")
	assert.Contains(t, printer.texts[0], "1-2-database-schema")
}
