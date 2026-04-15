package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"story-factory/internal/config"
	"story-factory/internal/output"
	"story-factory/internal/status"
)

// setupBatchStatus creates a sprint-status.yaml with multiple epics and stories.
// entries is a slice of [2]string{key, status} to preserve ordering in YAML output.
// epicKeys lists the epic entries (e.g., "epic-1", "epic-2") to include.
func setupBatchStatus(t *testing.T, tmpDir string, epicKeys []string, entries [][2]string) *status.Reader {
	t.Helper()
	statusDir := filepath.Join(tmpDir, "_bmad-output", "implementation-artifacts")
	require.NoError(t, os.MkdirAll(statusDir, 0o755))

	var sb string
	for _, ek := range epicKeys {
		sb += fmt.Sprintf("  %s: in-progress\n", ek)
	}
	for _, e := range entries {
		sb += fmt.Sprintf("  %s: %s\n", e[0], e[1])
	}

	content := fmt.Sprintf(`generated: 2026-01-01
last_updated: 2026-01-01
project: test
project_key: TEST
tracking_system: file-system
story_location: "{project-root}/_bmad-output/implementation-artifacts"

development_status:
%s`, sb)

	statusPath := filepath.Join(statusDir, "sprint-status.yaml")
	require.NoError(t, os.WriteFile(statusPath, []byte(content), 0o644))
	return status.NewReader(tmpDir)
}

// newBatchTestPipeline creates a Pipeline for batch tests with projectDir matching
// the status reader's basePath (required for fresh-reader re-read in runBatch).
func newBatchTestPipeline(t *testing.T, buf *bytes.Buffer, tmpDir string, reader *status.Reader) *Pipeline {
	t.Helper()
	cfg := config.DefaultConfig()
	printer := output.NewPrinterWithWriter(buf)
	return NewPipeline(nil, cfg, tmpDir,
		WithStatus(reader),
		WithPrinter(printer),
	)
}

// mockRunMap creates a runFunc from maps of results and errors keyed by story key.
// Successful results automatically get StepsExecuted populated so batch
// accounting can attribute step counts correctly.
func mockRunMap(results map[string]StoryResult, errs map[string]error) runFunc {
	return func(_ context.Context, key string) (StoryResult, error) {
		if errs != nil {
			if err, ok := errs[key]; ok {
				return StoryResult{}, err
			}
		}
		if results != nil {
			if r, ok := results[key]; ok {
				if r.Success && len(r.StepsExecuted) == 0 {
					r.StepsExecuted = []string{stepNameCreate, stepNameDevStory, stepNameCodeReview}
				}
				return r, nil
			}
		}
		return StoryResult{
			Key:           key,
			Success:       true,
			StepsExecuted: []string{stepNameCreate, stepNameDevStory, stepNameCodeReview},
		}, nil
	}
}

// --- RunEpic tests ---

func TestRunEpic(t *testing.T) {
	tests := []struct {
		name        string
		epicNum     int
		entries     [][2]string
		runResults  map[string]StoryResult
		runErrors   map[string]error
		wantLen     int
		wantCreated int
		wantFailed  int
		wantSkipped int
		wantErr     bool
	}{
		{
			name:    "happy path: all backlog stories succeed",
			epicNum: 1,
			entries: [][2]string{
				{"1-1-scaffold", string(status.StatusBacklog)},
				{"1-2-schema", string(status.StatusBacklog)},
				{"1-3-registration", string(status.StatusBacklog)},
			},
			runResults: map[string]StoryResult{
				"1-1-scaffold":     {Key: "1-1-scaffold", Success: true},
				"1-2-schema":       {Key: "1-2-schema", Success: true},
				"1-3-registration": {Key: "1-3-registration", Success: true},
			},
			wantLen:     3,
			wantCreated: 3,
		},
		{
			name:    "mixed statuses: only backlog stories are run",
			epicNum: 1,
			entries: [][2]string{
				{"1-1-scaffold", string(status.StatusBacklog)},
				{"1-2-schema", string(status.StatusReadyForDev)},
				{"1-3-registration", string(status.StatusBacklog)},
			},
			runResults: map[string]StoryResult{
				"1-1-scaffold":     {Key: "1-1-scaffold", Success: true},
				"1-3-registration": {Key: "1-3-registration", Success: true},
			},
			wantLen:     2,
			wantCreated: 2,
		},
		{
			name:    "failure isolation: one story fails, others still process",
			epicNum: 1,
			entries: [][2]string{
				{"1-1-scaffold", string(status.StatusBacklog)},
				{"1-2-schema", string(status.StatusBacklog)},
				{"1-3-registration", string(status.StatusBacklog)},
			},
			runResults: map[string]StoryResult{
				"1-1-scaffold":     {Key: "1-1-scaffold", Success: true},
				"1-2-schema":       {Key: "1-2-schema", FailedAt: "validate", Reason: "converge failed"},
				"1-3-registration": {Key: "1-3-registration", Success: true},
			},
			wantLen:     3,
			wantCreated: 2,
			wantFailed:  1,
		},
		{
			name:    "empty epic: no stories returns empty BatchResult",
			epicNum: 99,
			entries: [][2]string{},
			wantLen: 0,
		},
		{
			name:    "infrastructure error converted to failed StoryResult",
			epicNum: 1,
			entries: [][2]string{
				{"1-1-scaffold", string(status.StatusBacklog)},
				{"1-2-schema", string(status.StatusBacklog)},
			},
			runResults: map[string]StoryResult{
				"1-1-scaffold": {Key: "1-1-scaffold", Success: true},
			},
			runErrors: map[string]error{
				"1-2-schema": fmt.Errorf("disk full"),
			},
			wantLen:     2,
			wantCreated: 1,
			wantFailed:  1,
		},
		{
			name:    "all non-backlog: empty BatchResult, run not invoked",
			epicNum: 1,
			entries: [][2]string{
				{"1-1-scaffold", string(status.StatusReadyForDev)},
				{"1-2-schema", string(status.StatusDone)},
			},
			wantLen:     0,
			wantSkipped: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			reader := setupBatchStatus(t, tmpDir, []string{"epic-1", "epic-99"}, tt.entries)

			var buf bytes.Buffer
			p := newBatchTestPipeline(t, &buf, tmpDir, reader)

			mockRun := mockRunMap(tt.runResults, tt.runErrors)

			result, err := p.runEpic(context.Background(), tt.epicNum, mockRun)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tt.wantLen, len(result.Stories))
			assert.Equal(t, tt.epicNum, result.EpicNum)
			assert.Equal(t, tt.wantCreated, result.StepCounts[stepNameCreate])
			assert.Equal(t, tt.wantFailed, result.Failed)
			assert.Equal(t, tt.wantSkipped, result.Skipped)

			if tt.wantCreated > 0 {
				assert.Equal(t, tt.wantCreated, result.StepCounts[stepNameDevStory])
				assert.Equal(t, tt.wantCreated, result.StepCounts[stepNameCodeReview])
			}

			if tt.wantLen > 0 {
				assert.True(t, result.Duration > 0)
			}
		})
	}
}

func TestRunEpic_KeyOrder(t *testing.T) {
	tmpDir := t.TempDir()
	reader := setupBatchStatus(t, tmpDir, []string{"epic-1"}, [][2]string{
		{"1-3-registration", string(status.StatusBacklog)},
		{"1-1-scaffold", string(status.StatusBacklog)},
		{"1-2-schema", string(status.StatusBacklog)},
	})

	var buf bytes.Buffer
	p := newBatchTestPipeline(t, &buf, tmpDir, reader)

	var callOrder []string
	mockRun := func(_ context.Context, key string) (StoryResult, error) {
		callOrder = append(callOrder, key)
		return StoryResult{Key: key, Success: true}, nil
	}

	_, err := p.runEpic(context.Background(), 1, mockRun)
	require.NoError(t, err)

	assert.Equal(t, []string{"1-1-scaffold", "1-2-schema", "1-3-registration"}, callOrder,
		"stories should be processed in story number order")
}

func TestRunEpic_PrinterMessages(t *testing.T) {
	tmpDir := t.TempDir()
	reader := setupBatchStatus(t, tmpDir, []string{"epic-1"}, [][2]string{
		{"1-1-scaffold", string(status.StatusBacklog)},
		{"1-2-schema", string(status.StatusBacklog)},
	})

	var buf bytes.Buffer
	p := newBatchTestPipeline(t, &buf, tmpDir, reader)

	mockRun := func(_ context.Context, key string) (StoryResult, error) {
		return StoryResult{Key: key, Success: true}, nil
	}

	_, err := p.runEpic(context.Background(), 1, mockRun)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Processing epic 1: 2 backlog stories")
	assert.Contains(t, out, "[1/2] 1-1-scaffold")
	assert.Contains(t, out, "[2/2] 1-2-schema")
}

func TestRunEpic_EmptyPrinterMessage(t *testing.T) {
	tmpDir := t.TempDir()
	reader := setupBatchStatus(t, tmpDir, []string{"epic-99"}, [][2]string{})

	var buf bytes.Buffer
	p := newBatchTestPipeline(t, &buf, tmpDir, reader)

	result, err := p.runEpic(context.Background(), 99, func(_ context.Context, _ string) (StoryResult, error) {
		t.Fatal("run should not be called for empty epic")
		return StoryResult{}, nil
	})
	require.NoError(t, err)

	assert.Equal(t, 0, len(result.Stories))
	assert.Equal(t, 99, result.EpicNum)
	assert.Contains(t, buf.String(), "No stories found for epic 99")
}

func TestRunEpic_NoBacklogDoesNotCallRun(t *testing.T) {
	tmpDir := t.TempDir()
	reader := setupBatchStatus(t, tmpDir, []string{"epic-1"}, [][2]string{
		{"1-1-scaffold", string(status.StatusReadyForDev)},
	})

	var buf bytes.Buffer
	p := newBatchTestPipeline(t, &buf, tmpDir, reader)

	result, err := p.RunEpic(context.Background(), 1)
	require.NoError(t, err)
	require.Empty(t, result.Stories)
	assert.Equal(t, 0, result.Skipped)
	assert.Contains(t, buf.String(), "No backlog stories found for epic 1")
}

// --- RunQueue tests ---

func TestRunQueue(t *testing.T) {
	tests := []struct {
		name        string
		epicKeys    []string
		entries     [][2]string
		runResults  map[string]StoryResult
		runErrors   map[string]error
		wantLen     int
		wantCreated int
		wantFailed  int
		wantSkipped int
		wantErr     bool
	}{
		{
			name:     "happy path: multiple epics with backlog stories",
			epicKeys: []string{"epic-1", "epic-2", "epic-3"},
			entries: [][2]string{
				{"1-1-scaffold", string(status.StatusBacklog)},
				{"1-2-schema", string(status.StatusBacklog)},
				{"2-1-api", string(status.StatusBacklog)},
				{"2-2-auth", string(status.StatusBacklog)},
				{"2-3-db", string(status.StatusBacklog)},
				{"3-1-deploy", string(status.StatusBacklog)},
			},
			runResults: map[string]StoryResult{
				"1-1-scaffold": {Key: "1-1-scaffold", Success: true},
				"1-2-schema":   {Key: "1-2-schema", Success: true},
				"2-1-api":      {Key: "2-1-api", Success: true},
				"2-2-auth":     {Key: "2-2-auth", Success: true},
				"2-3-db":       {Key: "2-3-db", Success: true},
				"3-1-deploy":   {Key: "3-1-deploy", Success: true},
			},
			wantLen:     6,
			wantCreated: 6,
		},
		{
			name:     "mixed statuses: only backlog stories processed",
			epicKeys: []string{"epic-1", "epic-2"},
			entries: [][2]string{
				{"1-1-scaffold", string(status.StatusDone)},
				{"1-2-schema", string(status.StatusBacklog)},
				{"2-1-api", string(status.StatusReadyForDev)},
				{"2-2-auth", string(status.StatusBacklog)},
			},
			runResults: map[string]StoryResult{
				"1-2-schema": {Key: "1-2-schema", Success: true},
				"2-2-auth":   {Key: "2-2-auth", Success: true},
			},
			wantLen:     2,
			wantCreated: 2,
		},
		{
			name:     "empty backlog: returns BatchResult with zero stories",
			epicKeys: []string{"epic-1"},
			entries: [][2]string{
				{"1-1-scaffold", string(status.StatusDone)},
			},
			wantLen: 0,
		},
		{
			name:     "one story fails, rest continue",
			epicKeys: []string{"epic-1", "epic-2"},
			entries: [][2]string{
				{"1-1-scaffold", string(status.StatusBacklog)},
				{"2-1-api", string(status.StatusBacklog)},
				{"2-2-auth", string(status.StatusBacklog)},
			},
			runResults: map[string]StoryResult{
				"1-1-scaffold": {Key: "1-1-scaffold", Success: true},
				"2-1-api":      {Key: "2-1-api", FailedAt: "create", Reason: "timed out"},
				"2-2-auth":     {Key: "2-2-auth", Success: true},
			},
			wantLen:     3,
			wantCreated: 2,
			wantFailed:  1,
		},
		{
			name:     "infrastructure error: recorded as failure, batch continues",
			epicKeys: []string{"epic-1"},
			entries: [][2]string{
				{"1-1-scaffold", string(status.StatusBacklog)},
				{"1-2-schema", string(status.StatusBacklog)},
			},
			runResults: map[string]StoryResult{
				"1-2-schema": {Key: "1-2-schema", Success: true},
			},
			runErrors: map[string]error{
				"1-1-scaffold": fmt.Errorf("connection timeout"),
			},
			wantLen:     2,
			wantCreated: 1,
			wantFailed:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			reader := setupBatchStatus(t, tmpDir, tt.epicKeys, tt.entries)

			var buf bytes.Buffer
			p := newBatchTestPipeline(t, &buf, tmpDir, reader)

			mockRun := mockRunMap(tt.runResults, tt.runErrors)

			result, err := p.runQueue(context.Background(), mockRun)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tt.wantLen, len(result.Stories))
			assert.Equal(t, 0, result.EpicNum, "queue should have EpicNum=0")
			assert.Equal(t, tt.wantCreated, result.StepCounts[stepNameCreate])
			assert.Equal(t, tt.wantFailed, result.Failed)
			assert.Equal(t, tt.wantSkipped, result.Skipped)

			if tt.wantCreated > 0 {
				assert.Equal(t, tt.wantCreated, result.StepCounts[stepNameDevStory])
				assert.Equal(t, tt.wantCreated, result.StepCounts[stepNameCodeReview])
			}

			if tt.wantLen > 0 {
				assert.True(t, result.Duration > 0)
			}
		})
	}
}

func TestRunQueue_CrossEpicOrdering(t *testing.T) {
	tmpDir := t.TempDir()
	reader := setupBatchStatus(t, tmpDir,
		[]string{"epic-1", "epic-2", "epic-3"},
		[][2]string{
			// Deliberately out of order in YAML to prove sorting works
			{"3-1-deploy", string(status.StatusBacklog)},
			{"1-2-schema", string(status.StatusBacklog)},
			{"2-1-api", string(status.StatusBacklog)},
			{"1-1-scaffold", string(status.StatusBacklog)},
			{"2-2-auth", string(status.StatusBacklog)},
		},
	)

	var buf bytes.Buffer
	p := newBatchTestPipeline(t, &buf, tmpDir, reader)

	var callOrder []string
	mockRun := func(_ context.Context, key string) (StoryResult, error) {
		callOrder = append(callOrder, key)
		return StoryResult{Key: key, Success: true}, nil
	}

	_, err := p.runQueue(context.Background(), mockRun)
	require.NoError(t, err)

	assert.Equal(t,
		[]string{"1-1-scaffold", "1-2-schema", "2-1-api", "2-2-auth", "3-1-deploy"},
		callOrder,
		"stories should be processed in epic→story order regardless of YAML order")
}

func TestRunQueue_EmptyBacklogPrinterMessage(t *testing.T) {
	tmpDir := t.TempDir()
	reader := setupBatchStatus(t, tmpDir, []string{"epic-1"}, [][2]string{
		{"1-1-scaffold", string(status.StatusDone)},
	})

	var buf bytes.Buffer
	p := newBatchTestPipeline(t, &buf, tmpDir, reader)

	result, err := p.runQueue(context.Background(), func(_ context.Context, _ string) (StoryResult, error) {
		t.Fatal("run should not be called for empty backlog")
		return StoryResult{}, nil
	})
	require.NoError(t, err)

	assert.Equal(t, 0, len(result.Stories))
	assert.Contains(t, buf.String(), "No backlog stories found")
}

func TestRunQueue_SkipsStoriesNoLongerBacklog(t *testing.T) {
	// Simulate resumable batch: story is backlog in initial read but changes
	// status between stories. The fresh-reader re-read catches this.
	tmpDir := t.TempDir()
	reader := setupBatchStatus(t, tmpDir, []string{"epic-1"}, [][2]string{
		{"1-1-scaffold", string(status.StatusBacklog)},
		{"1-2-schema", string(status.StatusBacklog)},
	})

	var buf bytes.Buffer
	p := newBatchTestPipeline(t, &buf, tmpDir, reader)

	callCount := 0
	mockRun := func(_ context.Context, key string) (StoryResult, error) {
		callCount++
		if key == "1-1-scaffold" {
			// After processing 1-1, change 1-2's status to simulate BMAD update
			statusPath := filepath.Join(tmpDir, "_bmad-output", "implementation-artifacts", "sprint-status.yaml")
			data, err := os.ReadFile(statusPath)
			if err != nil {
				return StoryResult{}, err
			}
			updated := bytes.Replace(data, []byte("1-2-schema: backlog"), []byte("1-2-schema: ready-for-dev"), 1)
			if err := os.WriteFile(statusPath, updated, 0o644); err != nil {
				return StoryResult{}, err
			}
		}
		return StoryResult{
			Key:           key,
			Success:       true,
			StepsExecuted: []string{stepNameCreate, stepNameDevStory, stepNameCodeReview},
		}, nil
	}

	result, err := p.runQueue(context.Background(), mockRun)
	require.NoError(t, err)

	assert.Equal(t, 1, callCount, "only first story should be run; second skipped on re-read")
	require.Len(t, result.Stories, 2)
	assert.True(t, result.Stories[0].Success)
	assert.True(t, result.Stories[1].Skipped, "1-2-schema should be skipped after status change")
	assert.Equal(t, 1, result.StepCounts[stepNameCreate])
	assert.Equal(t, 1, result.Skipped)
}

func TestRunQueue_PrinterMessages(t *testing.T) {
	tmpDir := t.TempDir()
	reader := setupBatchStatus(t, tmpDir, []string{"epic-1"}, [][2]string{
		{"1-1-scaffold", string(status.StatusBacklog)},
		{"1-2-schema", string(status.StatusBacklog)},
	})

	var buf bytes.Buffer
	p := newBatchTestPipeline(t, &buf, tmpDir, reader)

	mockRun := func(_ context.Context, key string) (StoryResult, error) {
		return StoryResult{Key: key, Success: true}, nil
	}

	_, err := p.runQueue(context.Background(), mockRun)
	require.NoError(t, err)

	out := buf.String()
	assert.Contains(t, out, "Processing queue: 2 backlog stories")
	assert.Contains(t, out, "[1/2] 1-1-scaffold")
	assert.Contains(t, out, "[2/2] 1-2-schema")
}
