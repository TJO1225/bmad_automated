package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"story-factory/internal/config"
	"story-factory/internal/output"
	"story-factory/internal/status"
)

// mockStep returns a stepFunc that returns the given results in sequence.
// Each call consumes one element from the slice. If calls exceed the slice
// length, it panics (test programming error).
func mockStep(results []StepResult, errs []error) stepFunc {
	idx := 0
	return func(_ context.Context, _ string) (StepResult, error) {
		i := idx
		idx++
		var err error
		if i < len(errs) {
			err = errs[i]
		}
		return results[i], err
	}
}

// successStep returns a stepFunc that always succeeds with the given name.
func successStep(name string) stepFunc {
	return func(_ context.Context, _ string) (StepResult, error) {
		return StepResult{Name: name, Success: true, Duration: time.Millisecond}, nil
	}
}

// successValidateStep returns a stepFunc that succeeds with ValidationLoops set.
func successValidateStep(loops int) stepFunc {
	return func(_ context.Context, _ string) (StepResult, error) {
		return StepResult{
			Name:            "validate",
			Success:         true,
			Duration:        time.Millisecond,
			ValidationLoops: loops,
		}, nil
	}
}

// successSyncStep returns a stepFunc that succeeds with BeadID set.
func successSyncStep(beadID string) stepFunc {
	return func(_ context.Context, _ string) (StepResult, error) {
		return StepResult{
			Name:     "sync",
			Success:  true,
			BeadID:   beadID,
			Duration: time.Millisecond,
		}, nil
	}
}

// failStep returns a stepFunc that always fails operationally.
func failStep(name, reason string) stepFunc {
	return func(_ context.Context, _ string) (StepResult, error) {
		return StepResult{Name: name, Success: false, Reason: reason, Duration: time.Millisecond}, nil
	}
}

// errorStep returns a stepFunc that always returns an infrastructure error.
func errorStep(err error) stepFunc {
	return func(_ context.Context, _ string) (StepResult, error) {
		return StepResult{}, err
	}
}

// setupSprintStatus creates a sprint-status.yaml in tmpDir with the given
// story key and status. Returns a *status.Reader pointing at the file.
func setupRunStatus(t *testing.T, tmpDir, storyKey string, storyStatus status.Status) *status.Reader {
	t.Helper()
	statusDir := filepath.Join(tmpDir, "_bmad-output", "implementation-artifacts")
	require.NoError(t, os.MkdirAll(statusDir, 0o755))

	content := fmt.Sprintf(`generated: 2026-01-01
last_updated: 2026-01-01
project: test
project_key: TEST
tracking_system: file-system
story_location: "{project-root}/_bmad-output/implementation-artifacts"

development_status:
  epic-1: in-progress
  %s: %s
`, storyKey, storyStatus)

	statusPath := filepath.Join(statusDir, "sprint-status.yaml")
	require.NoError(t, os.WriteFile(statusPath, []byte(content), 0o644))

	return status.NewReader(tmpDir)
}

// newTestPipeline creates a Pipeline with a printer capturing to buf and
// the given status reader. The claude and beads executors are nil (tests
// use injected step functions via runPipeline).
func newTestPipeline(t *testing.T, buf *bytes.Buffer, reader *status.Reader) *Pipeline {
	t.Helper()
	cfg := config.DefaultConfig()
	printer := output.NewPrinterWithWriter(buf)
	return NewPipeline(nil, cfg, t.TempDir(),
		WithStatus(reader),
		WithPrinter(printer),
	)
}

func TestRunPipeline(t *testing.T) {
	tests := []struct {
		name        string
		storyStatus status.Status
		create      stepFunc
		validate    stepFunc
		sync        stepFunc
		wantSuccess bool
		wantSkipped bool
		wantFailAt  string
		wantLoops   int
		wantBeadID  string
		wantErr     bool
	}{
		{
			name:        "happy path: all steps succeed",
			storyStatus: status.StatusBacklog,
			create:      successStep("create"),
			validate:    successValidateStep(2),
			sync:        successSyncStep("bd-abc123"),
			wantSuccess: true,
			wantLoops:   2,
			wantBeadID:  "bd-abc123",
		},
		{
			name:        "skip: story not in backlog",
			storyStatus: status.StatusReadyForDev,
			create:      successStep("create"),
			validate:    successStep("validate"),
			sync:        successStep("sync"),
			wantSkipped: true,
		},
		{
			name:        "skip: story already done",
			storyStatus: status.StatusDone,
			create:      successStep("create"),
			validate:    successStep("validate"),
			sync:        successStep("sync"),
			wantSkipped: true,
		},
		{
			name:        "retry success at create: fails once then succeeds",
			storyStatus: status.StatusBacklog,
			create: mockStep(
				[]StepResult{
					{Name: "create", Success: false, Reason: "transient"},
					{Name: "create", Success: true, Duration: time.Millisecond},
				},
				[]error{nil, nil},
			),
			validate:    successValidateStep(1),
			sync:        successSyncStep("bd-retry"),
			wantSuccess: true,
			wantLoops:   1,
			wantBeadID:  "bd-retry",
		},
		{
			name:        "double failure at create",
			storyStatus: status.StatusBacklog,
			create: mockStep(
				[]StepResult{
					{Name: "create", Success: false, Reason: "fail-1"},
					{Name: "create", Success: false, Reason: "fail-2"},
				},
				[]error{nil, nil},
			),
			validate:   successStep("validate"),
			sync:       successStep("sync"),
			wantFailAt: "create",
		},
		{
			name:        "double failure at validate",
			storyStatus: status.StatusBacklog,
			create:      successStep("create"),
			validate: mockStep(
				[]StepResult{
					{Name: "validate", Success: false, Reason: "no-converge-1"},
					{Name: "validate", Success: false, Reason: "no-converge-2"},
				},
				[]error{nil, nil},
			),
			sync:       successStep("sync"),
			wantFailAt: "validate",
		},
		{
			name:        "sync operational failure: no retry (single attempt)",
			storyStatus: status.StatusBacklog,
			create:      successStep("create"),
			validate:    successValidateStep(1),
			sync: mockStep(
				[]StepResult{
					{Name: "sync", Success: false, Reason: "bd-fail-1"},
				},
				[]error{nil},
			),
			wantFailAt: "sync",
		},
		{
			name:        "infrastructure error at create: no retry",
			storyStatus: status.StatusBacklog,
			create:      errorStep(fmt.Errorf("disk full")),
			validate:    successStep("validate"),
			sync:        successStep("sync"),
			wantErr:     true,
		},
		{
			name:        "infrastructure error at validate: no retry",
			storyStatus: status.StatusBacklog,
			create:      successStep("create"),
			validate:    errorStep(fmt.Errorf("context canceled")),
			sync:        successStep("sync"),
			wantErr:     true,
		},
		{
			name:        "infrastructure error at sync: no retry",
			storyStatus: status.StatusBacklog,
			create:      successStep("create"),
			validate:    successValidateStep(1),
			sync:        errorStep(fmt.Errorf("permission denied")),
			wantErr:     true,
		},
		{
			name:        "retry at validate then succeeds",
			storyStatus: status.StatusBacklog,
			create:      successStep("create"),
			validate: mockStep(
				[]StepResult{
					{Name: "validate", Success: false, Reason: "transient"},
					{Name: "validate", Success: true, ValidationLoops: 3, Duration: time.Millisecond},
				},
				[]error{nil, nil},
			),
			sync:        successSyncStep("bd-after-retry"),
			wantSuccess: true,
			wantLoops:   3,
			wantBeadID:  "bd-after-retry",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			storyKey := "1-2-test-story"
			reader := setupRunStatus(t, t.TempDir(), storyKey, tt.storyStatus)
			p := newTestPipeline(t, &buf, reader)

			result, err := p.runPipeline(context.Background(), storyKey,
				tt.create, tt.validate, tt.sync)

			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			if tt.wantSkipped {
				assert.True(t, result.Skipped, "expected Skipped=true")
				assert.Equal(t, storyKey, result.Key)
				assert.Equal(t, string(tt.storyStatus), result.Reason, "skip should carry sprint status in Reason")
				return
			}

			assert.Equal(t, storyKey, result.Key)
			assert.Equal(t, tt.wantSuccess, result.Success)
			assert.Equal(t, tt.wantFailAt, result.FailedAt)
			assert.Equal(t, tt.wantLoops, result.ValidationLoops)
			assert.Equal(t, tt.wantBeadID, result.BeadID)

			if tt.wantSuccess {
				assert.True(t, result.Duration > 0, "duration should be positive")
			}
		})
	}
}

func TestRunStep_RetryOnce(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.DefaultConfig()
	printer := output.NewPrinterWithWriter(&buf)
	p := NewPipeline(nil, cfg, "",
		WithPrinter(printer),
	)

	callCount := 0
	step := func(_ context.Context, _ string) (StepResult, error) {
		callCount++
		if callCount == 1 {
			return StepResult{Name: "test", Success: false, Reason: "first-fail"}, nil
		}
		return StepResult{Name: "test", Success: true}, nil
	}

	result, err := p.runStep(context.Background(), "key", step)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 2, callCount, "step should be called twice (initial + retry)")
	assert.Contains(t, buf.String(), "Retrying test...")
}

func TestRunStep_NoRetryOnSuccess(t *testing.T) {
	p := NewPipeline(nil, config.DefaultConfig(), "")

	callCount := 0
	step := func(_ context.Context, _ string) (StepResult, error) {
		callCount++
		return StepResult{Name: "test", Success: true}, nil
	}

	result, err := p.runStep(context.Background(), "key", step)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, callCount, "successful step should not be retried")
}

func TestRunStep_NoRetryOnInfrastructureError(t *testing.T) {
	p := NewPipeline(nil, config.DefaultConfig(), "")

	callCount := 0
	step := func(_ context.Context, _ string) (StepResult, error) {
		callCount++
		return StepResult{}, fmt.Errorf("infra error")
	}

	_, err := p.runStep(context.Background(), "key", step)
	require.Error(t, err)
	assert.Equal(t, 1, callCount, "infrastructure error should not be retried")
}

func TestRunStep_NoRetryOnSyncOperationalFailure(t *testing.T) {
	p := NewPipeline(nil, config.DefaultConfig(), "")

	callCount := 0
	step := func(_ context.Context, _ string) (StepResult, error) {
		callCount++
		if callCount == 1 {
			return StepResult{Name: stepNameSync, Success: false, Reason: "append failed"}, nil
		}
		return StepResult{Name: stepNameSync, Success: true, BeadID: "bd-would-win"}, nil
	}

	result, err := p.runStep(context.Background(), "key", step)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, "append failed", result.Reason)
	assert.Equal(t, 1, callCount, "sync operational failure must not retry (avoids duplicate bd create)")
}

func TestRun_StoryNotFound(t *testing.T) {
	var buf bytes.Buffer
	reader := setupRunStatus(t, t.TempDir(), "other-story", status.StatusBacklog)
	p := newTestPipeline(t, &buf, reader)

	_, err := p.Run(context.Background(), "nonexistent-story")
	require.Error(t, err, "should return error for unknown story key")
}

func TestRunPipeline_PrinterMessages(t *testing.T) {
	var buf bytes.Buffer
	storyKey := "1-2-test"
	reader := setupRunStatus(t, t.TempDir(), storyKey, status.StatusBacklog)
	p := newTestPipeline(t, &buf, reader)

	_, err := p.runPipeline(context.Background(), storyKey,
		successStep("create"),
		successValidateStep(1),
		successSyncStep("bd-123"),
	)
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Starting pipeline")
	assert.Contains(t, output, "create")
	assert.Contains(t, output, "validate")
	assert.Contains(t, output, "sync")
}

func TestRunPipeline_DoubleFailureStopsEarly(t *testing.T) {
	storyKey := "1-2-test"
	reader := setupRunStatus(t, t.TempDir(), storyKey, status.StatusBacklog)

	validateCalled := false
	syncCalled := false

	var buf bytes.Buffer
	p := newTestPipeline(t, &buf, reader)

	result, err := p.runPipeline(context.Background(), storyKey,
		failStep("create", "always-fail"),
		func(_ context.Context, _ string) (StepResult, error) {
			validateCalled = true
			return StepResult{Name: "validate", Success: true}, nil
		},
		func(_ context.Context, _ string) (StepResult, error) {
			syncCalled = true
			return StepResult{Name: "sync", Success: true}, nil
		},
	)

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, "create", result.FailedAt)
	assert.False(t, validateCalled, "validate should not be called after create failure")
	assert.False(t, syncCalled, "sync should not be called after create failure")
}
