package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"story-factory/internal/claude"
	"story-factory/internal/config"
	"story-factory/internal/status"
)

// writeSprintStatus writes a minimal sprint-status.yaml with the given
// key/status pair so status.NewReader(dir) can resolve it.
func writeSprintStatus(t *testing.T, dir, key, stat string) {
	t.Helper()
	statusDir := filepath.Dir(filepath.Join(dir, status.DefaultStatusPath))
	require.NoError(t, os.MkdirAll(statusDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, status.DefaultStatusPath),
		[]byte(`story_location: "{project-root}/_bmad-output/implementation-artifacts"
development_status:
  `+key+`: `+stat+"\n"),
		0644,
	))
}

func TestStepCreate_ResumeSkipsWhenAlreadyPastBacklog(t *testing.T) {
	cases := []string{"ready-for-dev", "in-progress", "review", "done"}
	for _, s := range cases {
		t.Run(s, func(t *testing.T) {
			dir := t.TempDir()
			key := "1-2-resume"
			writeSprintStatus(t, dir, key, s)

			mock := &claude.MockExecutor{ExitCode: 0}
			p := NewPipeline(mock, config.DefaultConfig(), dir,
				WithStatus(status.NewReader(dir)),
			)

			result, err := p.StepCreate(context.Background(), key)
			require.NoError(t, err)
			assert.True(t, result.Success)
			assert.Contains(t, result.Reason, "already complete")
			assert.Empty(t, mock.RecordedPrompts, "Claude must not be invoked on resume skip")
		})
	}
}

func TestStepDevStory_ResumeSkipsWhenPastReview(t *testing.T) {
	for _, s := range []string{"review", "done"} {
		t.Run(s, func(t *testing.T) {
			dir := t.TempDir()
			key := "1-2-resume"
			writeSprintStatus(t, dir, key, s)

			mock := &claude.MockExecutor{ExitCode: 0}
			p := NewPipeline(mock, config.DefaultConfig(), dir,
				WithStatus(status.NewReader(dir)),
			)

			result, err := p.StepDevStory(context.Background(), key)
			require.NoError(t, err)
			assert.True(t, result.Success)
			assert.Contains(t, result.Reason, "already complete")
			assert.Empty(t, mock.RecordedPrompts, "Claude must not be invoked on resume skip")
		})
	}
}

func TestStepDevStory_FailsWhenStillBacklog(t *testing.T) {
	dir := t.TempDir()
	key := "1-2-not-ready"
	writeSprintStatus(t, dir, key, "backlog")

	mock := &claude.MockExecutor{ExitCode: 0}
	p := NewPipeline(mock, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
	)

	result, err := p.StepDevStory(context.Background(), key)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Reason, "run create-story first")
	assert.Empty(t, mock.RecordedPrompts)
}

func TestStepCodeReview_ResumeSkipsWhenDone(t *testing.T) {
	dir := t.TempDir()
	key := "1-2-resume"
	writeSprintStatus(t, dir, key, "done")

	mock := &claude.MockExecutor{ExitCode: 0}
	p := NewPipeline(mock, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
	)

	result, err := p.StepCodeReview(context.Background(), key)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Reason, "already complete")
	assert.Empty(t, mock.RecordedPrompts)
}

func TestStepCodeReview_FailsWhenNotYetReview(t *testing.T) {
	dir := t.TempDir()
	key := "1-2-not-ready"
	writeSprintStatus(t, dir, key, "in-progress")

	mock := &claude.MockExecutor{ExitCode: 0}
	p := NewPipeline(mock, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
	)

	result, err := p.StepCodeReview(context.Background(), key)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Reason, "expected review")
	assert.Empty(t, mock.RecordedPrompts)
}
