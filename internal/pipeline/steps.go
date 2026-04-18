package pipeline

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"story-factory/internal/beads"
	"story-factory/internal/config"
	"story-factory/internal/executor"
	"story-factory/internal/status"
)

// Step name constants — these must match the keys in [config.Config.Modes]
// step lists and the names registered in [Pipeline.stepRegistry].
const (
	stepNameCreate       = "create-story"
	stepNameDevStory     = "dev-story"
	stepNameCodeReview   = "code-review"
	stepNameSync         = "sync-to-beads"
	stepNameCommitBranch = "commit-branch"
	stepNameOpenPR       = "open-pr"
	stepNameReviewPR     = "review-pr"

	// DefaultTimeout is the maximum duration for a single LLM subprocess
	// (create-story, dev-story, code-review, review-pr). Dev stories often run
	// full frontend/backend test suites; 90m avoids spurious kills. Override with
	// BMAD_PIPELINE_STEP_TIMEOUT or [WithLLMStepTimeout] for longer runs.
	DefaultTimeout = 90 * time.Minute
)

// Pipeline orchestrates multi-step story processing workflows.
//
// Each step method (StepCreate, StepDevStory, StepCodeReview, stepSync) runs
// one phase of the lifecycle. The Pipeline holds injected dependencies so
// steps can invoke LLM backends, read configuration, and resolve file paths.
//
// The pipeline supports multiple backends via [WithExecutors]. Each step
// resolves its executor through [Pipeline.resolveExecutor], which checks
// (in order): workflow-level backend override, mode-level default backend,
// then the pipeline's default executor.
type Pipeline struct {
	defaultExecutor executor.Executor
	executors       map[string]executor.Executor
	beads           beads.Executor
	status          *status.Reader
	printer         Printer
	cfg             *config.Config
	projectDir      string
	mode            string
	dryRun          bool
	verbose         bool
	// llmStepTimeout caps each LLM subprocess (create-story, dev-story, code-review, review-pr).
	// Zero means use [DefaultTimeout]. Override via [WithLLMStepTimeout] or
	// BMAD_PIPELINE_STEP_TIMEOUT (see [LLMStepTimeoutFromEnv]).
	llmStepTimeout time.Duration
}

// NewPipeline creates a Pipeline with the given dependencies.
//
// The defaultExec is used for all LLM-driven steps unless overridden by
// per-step backend configuration. Use [WithExecutors] to provide additional
// named backends.
func NewPipeline(defaultExec executor.Executor, cfg *config.Config, projectDir string, opts ...PipelineOption) *Pipeline {
	p := &Pipeline{
		defaultExecutor: defaultExec,
		executors:       make(map[string]executor.Executor),
		cfg:             cfg,
		projectDir:      projectDir,
		mode:            config.ModeBmad,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

// resolveExecutor returns the appropriate executor for the given step name.
//
// Resolution order:
//  1. Workflow-level backend override (WorkflowConfig.Backend)
//  2. Mode-level default backend (ModeConfig.DefaultBackend)
//  3. Pipeline's default executor
func (p *Pipeline) resolveExecutor(stepName string) executor.Executor {
	// Check workflow-level backend override
	if wf, ok := p.cfg.Workflows[stepName]; ok && wf.Backend != "" {
		if exec, ok := p.executors[wf.Backend]; ok {
			return exec
		}
	}
	// Check mode-level default backend
	if mode, ok := p.cfg.Modes[p.mode]; ok && mode.DefaultBackend != "" {
		if exec, ok := p.executors[mode.DefaultBackend]; ok {
			return exec
		}
	}
	return p.defaultExecutor
}

// resolveBackendName returns the active backend name for a step.
// Used to select per-backend prompt templates.
func (p *Pipeline) resolveBackendName(stepName string) string {
	if wf, ok := p.cfg.Workflows[stepName]; ok && wf.Backend != "" {
		return wf.Backend
	}
	if mode, ok := p.cfg.Modes[p.mode]; ok && mode.DefaultBackend != "" {
		return mode.DefaultBackend
	}
	return ""
}

// PipelineOption configures optional Pipeline dependencies.
type PipelineOption func(*Pipeline)

// WithStatus sets the status reader.
func WithStatus(r *status.Reader) PipelineOption {
	return func(p *Pipeline) { p.status = r }
}

// WithPrinter sets the printer.
func WithPrinter(pr Printer) PipelineOption {
	return func(p *Pipeline) { p.printer = pr }
}

// WithBeads sets the beads executor for syncing stories to Gastown Beads.
func WithBeads(b beads.Executor) PipelineOption {
	return func(p *Pipeline) { p.beads = b }
}

// WithExecutors sets named backend executors for per-step backend selection.
// Keys are backend names (e.g., "claude", "gemini", "cursor") matching
// [config.BackendConfig] keys.
func WithExecutors(executors map[string]executor.Executor) PipelineOption {
	return func(p *Pipeline) {
		for k, v := range executors {
			p.executors[k] = v
		}
	}
}

// WithDryRun enables dry-run mode.
func WithDryRun(v bool) PipelineOption {
	return func(p *Pipeline) { p.dryRun = v }
}

// WithVerbose enables verbose output.
func WithVerbose(v bool) PipelineOption {
	return func(p *Pipeline) { p.verbose = v }
}

// WithMode sets the pipeline mode (e.g. [config.ModeBmad], [config.ModeBeads]).
// If unset, the pipeline defaults to [config.ModeBmad].
func WithMode(mode string) PipelineOption {
	return func(p *Pipeline) {
		if mode != "" {
			p.mode = mode
		}
	}
}

// WithLLMStepTimeout sets the maximum duration for a single Claude (or other
// backend) subprocess in create-story, dev-story, code-review, and review-pr.
// Values <= 0 are ignored so the default [DefaultTimeout] applies.
func WithLLMStepTimeout(d time.Duration) PipelineOption {
	return func(p *Pipeline) {
		if d > 0 {
			p.llmStepTimeout = d
		}
	}
}

// llmTimeout returns the effective per-step LLM timeout.
func (p *Pipeline) llmTimeout() time.Duration {
	if p.llmStepTimeout > 0 {
		return p.llmStepTimeout
	}
	return DefaultTimeout
}

// StepCreate runs the story creation step: invoke Claude to create a story file
// from a backlog entry, then verify post-conditions (file exists, status changed).
//
// Returns a StepResult for operational outcomes and an error for infrastructure failures.
func (p *Pipeline) StepCreate(ctx context.Context, key string) (StepResult, error) {
	start := time.Now()

	if p.dryRun {
		msg := "dry-run: would create story " + key
		if p.printer != nil {
			p.printer.Text(msg)
		}
		return StepResult{
			Name:    stepNameCreate,
			Success: true,
			Reason:  msg,
		}, nil
	}

	if p.status == nil {
		return StepResult{}, fmt.Errorf("create story %s: pipeline has no status reader", key)
	}

	// Resume skip: if the story is already past backlog, create-story has
	// already run in a previous session. Return success without invoking
	// Claude so the pipeline can advance to dev-story / code-review.
	preReader := status.NewReader(p.projectDir)
	preEntry, err := preReader.StoryByKey(key)
	if err == nil && preEntry.Status != status.StatusBacklog {
		msg := fmt.Sprintf("create-story already complete (status: %s)", preEntry.Status)
		if p.printer != nil {
			p.printer.Text(msg)
		}
		return StepResult{
			Name:     stepNameCreate,
			Success:  true,
			Reason:   msg,
			Duration: time.Since(start),
		}, nil
	}

	prompt, err := p.cfg.GetPrompt("create-story", key)
	if err != nil {
		return StepResult{}, fmt.Errorf("create story %s: %w", key, err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, p.llmTimeout())
	defer cancel()

	var handler executor.EventHandler
	if p.verbose && p.printer != nil {
		handler = func(event executor.Event) {
			if event.IsText() {
				p.printer.Text(event.Text)
			}
		}
	}

	exec := p.resolveExecutor(stepNameCreate)
	exitCode, err := exec.ExecuteWithResult(timeoutCtx, prompt, handler)
	if err != nil {
		switch {
		case errors.Is(err, context.DeadlineExceeded) || errors.Is(timeoutCtx.Err(), context.DeadlineExceeded):
			return StepResult{
				Name:     stepNameCreate,
				Success:  false,
				Reason:   "create story " + key + ": timed out",
				Duration: time.Since(start),
			}, nil
		case errors.Is(err, context.Canceled) || errors.Is(timeoutCtx.Err(), context.Canceled):
			return StepResult{
				Name:     stepNameCreate,
				Success:  false,
				Reason:   "create story " + key + ": canceled",
				Duration: time.Since(start),
			}, nil
		default:
			return StepResult{}, err
		}
	}
	if exitCode != 0 {
		return StepResult{
			Name:     stepNameCreate,
			Success:  false,
			Reason:   fmt.Sprintf("create story %s: claude exited with code %d", key, exitCode),
			Duration: time.Since(start),
		}, nil
	}

	// Post-condition 1: story file exists
	storyDir, err := p.status.ResolveStoryLocation(p.projectDir)
	if err != nil {
		return StepResult{}, fmt.Errorf("create story %s: %w", key, err)
	}
	storyPath := filepath.Join(storyDir, key+".md")
	if _, err := os.Stat(storyPath); err != nil {
		return StepResult{
			Name:     stepNameCreate,
			Success:  false,
			Reason:   fmt.Sprintf("create story %s: story file not created at %s", key, storyPath),
			Duration: time.Since(start),
		}, nil
	}

	// Post-condition 2: status changed (fresh reader for fresh disk read)
	freshReader := status.NewReader(p.projectDir)
	entry, err := freshReader.StoryByKey(key)
	if err != nil {
		if errors.Is(err, status.ErrStoryNotFound) {
			return StepResult{
				Name:     stepNameCreate,
				Success:  false,
				Reason:   fmt.Sprintf("create story %s: story key not in sprint-status.yaml", key),
				Duration: time.Since(start),
			}, nil
		}
		return StepResult{}, fmt.Errorf("create story %s: read sprint status: %w", key, err)
	}
	if entry.Status == status.StatusBacklog {
		return StepResult{
			Name:     stepNameCreate,
			Success:  false,
			Reason:   fmt.Sprintf("create story %s: sprint status not updated", key),
			Duration: time.Since(start),
		}, nil
	}

	return StepResult{
		Name:     stepNameCreate,
		Success:  true,
		Duration: time.Since(start),
	}, nil
}

// StepDevStory and StepCodeReview are implemented in steps_bmad.go.

// stepSync synchronizes a validated story to Gastown Beads via the Pipeline's
// beads executor. Unlike the static [StepSync], this method has access to
// Pipeline fields for dry-run and verbose support.
func (p *Pipeline) stepSync(ctx context.Context, key string) (StepResult, error) {
	if p.dryRun {
		msg := "dry-run: would sync story " + key + " to beads"
		if p.printer != nil {
			p.printer.Text(msg)
		}
		return StepResult{
			Name:    stepNameSync,
			Success: true,
			Reason:  msg,
		}, nil
	}

	start := time.Now()

	if p.status == nil {
		return StepResult{}, fmt.Errorf("stepSync: pipeline has no status reader")
	}
	if p.beads == nil {
		return StepResult{}, fmt.Errorf("stepSync: pipeline has no beads executor")
	}

	storyDir, err := p.status.ResolveStoryLocation(p.projectDir)
	if err != nil {
		return StepResult{}, fmt.Errorf("stepSync: %w", err)
	}
	storyPath := filepath.Join(storyDir, key+".md")

	content, err := os.ReadFile(storyPath)
	if err != nil {
		return StepResult{Name: stepNameSync}, fmt.Errorf("read story file %s: %w", storyPath, err)
	}

	title, err := beads.ExtractTitle(string(content))
	if err != nil {
		return StepResult{
			Name:     stepNameSync,
			Success:  false,
			Reason:   fmt.Sprintf("extract title: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	var bdOut io.Writer
	var lineW *printerLineWriter
	if p.verbose && p.printer != nil {
		lineW = newPrinterLineWriter(p.printer)
		bdOut = lineW
	}

	beadID, err := p.beads.Create(ctx, key, title, storyPath, bdOut)
	if lineW != nil {
		lineW.flush()
	}
	if err != nil {
		return StepResult{
			Name:     stepNameSync,
			Success:  false,
			Reason:   fmt.Sprintf("bd create: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	if err := beads.AppendTrackingComment(storyPath, beadID); err != nil {
		return StepResult{
			Name:     stepNameSync,
			Success:  false,
			Reason:   fmt.Sprintf("append tracking comment: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	return StepResult{
		Name:     stepNameSync,
		Success:  true,
		BeadID:   beadID,
		Duration: time.Since(start),
	}, nil
}

// StepSync synchronizes a validated story to Gastown Beads.
//
// It reads the story file, extracts the title and acceptance criteria,
// invokes bd create via the provided [beads.Executor], and appends a
// tracking comment to the story file on success.
//
// Returns a [StepResult] for operational outcomes (parsing failures, bd
// failures) and an error for infrastructure failures (filesystem unreadable).
func StepSync(ctx context.Context, beadsExec beads.Executor, storyPath, key string) (StepResult, error) {
	start := time.Now()

	content, err := os.ReadFile(storyPath)
	if err != nil {
		return StepResult{Name: stepNameSync}, fmt.Errorf("read story file %s: %w", storyPath, err)
	}

	title, err := beads.ExtractTitle(string(content))
	if err != nil {
		return StepResult{
			Name:     stepNameSync,
			Success:  false,
			Reason:   fmt.Sprintf("extract title: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	acs, err := beads.ExtractAcceptanceCriteria(string(content))
	if err != nil {
		return StepResult{
			Name:     stepNameSync,
			Success:  false,
			Reason:   fmt.Sprintf("extract ACs: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	beadID, err := beadsExec.Create(ctx, key, title, acs, nil)
	if err != nil {
		return StepResult{
			Name:     stepNameSync,
			Success:  false,
			Reason:   fmt.Sprintf("bd create: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	if err := beads.AppendTrackingComment(storyPath, beadID); err != nil {
		return StepResult{
			Name:     stepNameSync,
			Success:  false,
			Reason:   fmt.Sprintf("append tracking comment: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	return StepResult{
		Name:     stepNameSync,
		Success:  true,
		BeadID:   beadID,
		Duration: time.Since(start),
	}, nil
}
