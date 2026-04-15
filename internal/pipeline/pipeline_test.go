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

// successSyncStep returns a stepFunc that succeeds with BeadID set.
func successSyncStep(beadID string) stepFunc {
	return func(_ context.Context, _ string) (StepResult, error) {
		return StepResult{
			Name:     stepNameSync,
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

// threeStepPipeline builds a generic 3-step pipeline [create, dev, sync]
// from bare stepFuncs. Naming matches registered steps so retry policy works.
func threeStepPipeline(create, dev, sync stepFunc) []namedStep {
	return []namedStep{
		{Name: stepNameCreate, Fn: create},
		{Name: stepNameDevStory, Fn: dev},
		{Name: stepNameSync, Fn: sync},
	}
}

func TestRunPipeline(t *testing.T) {
	tests := []struct {
		name        string
		storyStatus status.Status
		steps       []namedStep
		wantSuccess bool
		wantSkipped bool
		wantFailAt  string
		wantBeadID  string
		wantErr     bool
	}{
		{
			name:        "happy path: all steps succeed",
			storyStatus: status.StatusBacklog,
			steps: threeStepPipeline(
				successStep(stepNameCreate),
				successStep(stepNameDevStory),
				successSyncStep("bd-abc123"),
			),
			wantSuccess: true,
			wantBeadID:  "bd-abc123",
		},
		{
			name:        "skip: story already done",
			storyStatus: status.StatusDone,
			steps: threeStepPipeline(
				successStep(stepNameCreate),
				successStep(stepNameDevStory),
				successStep(stepNameSync),
			),
			wantSkipped: true,
		},
		{
			name:        "retry success at create: fails once then succeeds",
			storyStatus: status.StatusBacklog,
			steps: []namedStep{
				{Name: stepNameCreate, Fn: mockStep(
					[]StepResult{
						{Name: stepNameCreate, Success: false, Reason: "transient"},
						{Name: stepNameCreate, Success: true, Duration: time.Millisecond},
					},
					[]error{nil, nil},
				)},
				{Name: stepNameDevStory, Fn: successStep(stepNameDevStory)},
				{Name: stepNameSync, Fn: successSyncStep("bd-retry")},
			},
			wantSuccess: true,
			wantBeadID:  "bd-retry",
		},
		{
			name:        "double failure at create",
			storyStatus: status.StatusBacklog,
			steps: []namedStep{
				{Name: stepNameCreate, Fn: mockStep(
					[]StepResult{
						{Name: stepNameCreate, Success: false, Reason: "fail-1"},
						{Name: stepNameCreate, Success: false, Reason: "fail-2"},
					},
					[]error{nil, nil},
				)},
				{Name: stepNameDevStory, Fn: successStep(stepNameDevStory)},
				{Name: stepNameSync, Fn: successStep(stepNameSync)},
			},
			wantFailAt: stepNameCreate,
		},
		{
			name:        "sync operational failure: no retry (single attempt)",
			storyStatus: status.StatusBacklog,
			steps: threeStepPipeline(
				successStep(stepNameCreate),
				successStep(stepNameDevStory),
				mockStep(
					[]StepResult{
						{Name: stepNameSync, Success: false, Reason: "bd-fail-1"},
					},
					[]error{nil},
				),
			),
			wantFailAt: stepNameSync,
		},
		{
			name:        "infrastructure error at create: no retry",
			storyStatus: status.StatusBacklog,
			steps: threeStepPipeline(
				errorStep(fmt.Errorf("disk full")),
				successStep(stepNameDevStory),
				successStep(stepNameSync),
			),
			wantErr: true,
		},
		{
			name:        "infrastructure error at sync: no retry",
			storyStatus: status.StatusBacklog,
			steps: threeStepPipeline(
				successStep(stepNameCreate),
				successStep(stepNameDevStory),
				errorStep(fmt.Errorf("permission denied")),
			),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			storyKey := "1-2-test-story"
			reader := setupRunStatus(t, t.TempDir(), storyKey, tt.storyStatus)
			p := newTestPipeline(t, &buf, reader)

			result, err := p.runPipeline(context.Background(), storyKey, tt.steps)

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
			assert.Equal(t, tt.wantBeadID, result.BeadID)

			if tt.wantSuccess {
				assert.True(t, result.Duration > 0, "duration should be positive")
				assert.Equal(t, []string{stepNameCreate, stepNameDevStory, stepNameSync}, result.StepsExecuted)
			}
		})
	}
}

func TestRunPipeline_NeedsReviewSurfacedDistinctly(t *testing.T) {
	storyKey := "1-2-test-story"
	reader := setupRunStatus(t, t.TempDir(), storyKey, status.StatusBacklog)

	var buf bytes.Buffer
	p := newTestPipeline(t, &buf, reader)

	steps := []namedStep{
		{Name: stepNameCreate, Fn: successStep(stepNameCreate)},
		{Name: stepNameDevStory, Fn: successStep(stepNameDevStory)},
		{Name: stepNameCodeReview, Fn: func(_ context.Context, _ string) (StepResult, error) {
			return StepResult{
				Name:     stepNameCodeReview,
				Success:  false,
				Reason:   codeReviewNeedsReviewReason,
				Duration: time.Millisecond,
			}, nil
		}},
	}

	result, err := p.runPipeline(context.Background(), storyKey, steps)
	require.NoError(t, err)

	assert.False(t, result.Success)
	assert.True(t, result.NeedsReview, "NeedsReview should be set when code-review returns needs-review reason")
	assert.Equal(t, stepNameCodeReview, result.FailedAt)
	assert.Equal(t, codeReviewNeedsReviewReason, result.Reason)
}

func TestRunStep_RetryOnce(t *testing.T) {
	var buf bytes.Buffer
	cfg := config.DefaultConfig()
	printer := output.NewPrinterWithWriter(&buf)
	p := NewPipeline(nil, cfg, "",
		WithPrinter(printer),
	)

	callCount := 0
	step := namedStep{Name: stepNameCreate, Fn: func(_ context.Context, _ string) (StepResult, error) {
		callCount++
		if callCount == 1 {
			return StepResult{Name: stepNameCreate, Success: false, Reason: "first-fail"}, nil
		}
		return StepResult{Name: stepNameCreate, Success: true}, nil
	}}

	result, err := p.runStep(context.Background(), "key", step)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 2, callCount, "step should be called twice (initial + retry)")
	assert.Contains(t, buf.String(), "Retrying "+stepNameCreate+"...")
}

func TestRunStep_NoRetryOnSuccess(t *testing.T) {
	p := NewPipeline(nil, config.DefaultConfig(), "")

	callCount := 0
	step := namedStep{Name: stepNameCreate, Fn: func(_ context.Context, _ string) (StepResult, error) {
		callCount++
		return StepResult{Name: stepNameCreate, Success: true}, nil
	}}

	result, err := p.runStep(context.Background(), "key", step)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, 1, callCount, "successful step should not be retried")
}

func TestRunStep_NoRetryOnInfrastructureError(t *testing.T) {
	p := NewPipeline(nil, config.DefaultConfig(), "")

	callCount := 0
	step := namedStep{Name: stepNameCreate, Fn: func(_ context.Context, _ string) (StepResult, error) {
		callCount++
		return StepResult{}, fmt.Errorf("infra error")
	}}

	_, err := p.runStep(context.Background(), "key", step)
	require.Error(t, err)
	assert.Equal(t, 1, callCount, "infrastructure error should not be retried")
}

func TestRunStep_NoRetryOnSyncOperationalFailure(t *testing.T) {
	p := NewPipeline(nil, config.DefaultConfig(), "")

	callCount := 0
	step := namedStep{Name: stepNameSync, Fn: func(_ context.Context, _ string) (StepResult, error) {
		callCount++
		if callCount == 1 {
			return StepResult{Name: stepNameSync, Success: false, Reason: "append failed"}, nil
		}
		return StepResult{Name: stepNameSync, Success: true, BeadID: "bd-would-win"}, nil
	}}

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

func TestRun_UnknownMode(t *testing.T) {
	var buf bytes.Buffer
	reader := setupRunStatus(t, t.TempDir(), "1-2-test", status.StatusBacklog)
	p := newTestPipeline(t, &buf, reader)
	p.mode = "does-not-exist"

	_, err := p.Run(context.Background(), "1-2-test")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown mode")
}

func TestRunPipeline_PrinterMessages(t *testing.T) {
	var buf bytes.Buffer
	storyKey := "1-2-test"
	reader := setupRunStatus(t, t.TempDir(), storyKey, status.StatusBacklog)
	p := newTestPipeline(t, &buf, reader)

	_, err := p.runPipeline(context.Background(), storyKey, threeStepPipeline(
		successStep(stepNameCreate),
		successStep(stepNameDevStory),
		successSyncStep("bd-123"),
	))
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Starting pipeline")
	assert.Contains(t, output, stepNameCreate)
	assert.Contains(t, output, stepNameDevStory)
	assert.Contains(t, output, stepNameSync)
}

func TestRunPipeline_DoubleFailureStopsEarly(t *testing.T) {
	storyKey := "1-2-test"
	reader := setupRunStatus(t, t.TempDir(), storyKey, status.StatusBacklog)

	devCalled := false
	syncCalled := false

	var buf bytes.Buffer
	p := newTestPipeline(t, &buf, reader)

	steps := []namedStep{
		{Name: stepNameCreate, Fn: failStep(stepNameCreate, "always-fail")},
		{Name: stepNameDevStory, Fn: func(_ context.Context, _ string) (StepResult, error) {
			devCalled = true
			return StepResult{Name: stepNameDevStory, Success: true}, nil
		}},
		{Name: stepNameSync, Fn: func(_ context.Context, _ string) (StepResult, error) {
			syncCalled = true
			return StepResult{Name: stepNameSync, Success: true}, nil
		}},
	}

	result, err := p.runPipeline(context.Background(), storyKey, steps)

	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Equal(t, stepNameCreate, result.FailedAt)
	assert.False(t, devCalled, "dev-story should not be called after create-story failure")
	assert.False(t, syncCalled, "sync should not be called after create-story failure")
}
