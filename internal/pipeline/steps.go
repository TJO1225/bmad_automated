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
	"story-factory/internal/claude"
	"story-factory/internal/config"
	"story-factory/internal/status"
)

const (
	maxValidationLoops = 3
	stepNameValidate   = "validate"
	stepNameCreate     = "create"
	stepNameSync       = "sync"

	// DefaultTimeout is the maximum duration for a single Claude subprocess.
	// Set high enough for complex skills that do research (create-story, dev-story).
	DefaultTimeout = 15 * time.Minute
)

// Pipeline orchestrates multi-step story processing workflows.
//
// Each step method (StepCreate, StepValidate, stepSync) runs one phase
// of the lifecycle. The Pipeline holds injected dependencies so steps
// can invoke Claude, read configuration, and resolve file paths.
type Pipeline struct {
	claude     claude.Executor
	beads      beads.Executor
	status     *status.Reader
	printer    Printer
	cfg        *config.Config
	projectDir string
	dryRun     bool
	verbose    bool
}

// NewPipeline creates a Pipeline with the given dependencies.
func NewPipeline(executor claude.Executor, cfg *config.Config, projectDir string, opts ...PipelineOption) *Pipeline {
	p := &Pipeline{
		claude:     executor,
		cfg:        cfg,
		projectDir: projectDir,
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
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

// WithDryRun enables dry-run mode.
func WithDryRun(v bool) PipelineOption {
	return func(p *Pipeline) { p.dryRun = v }
}

// WithVerbose enables verbose output.
func WithVerbose(v bool) PipelineOption {
	return func(p *Pipeline) { p.verbose = v }
}

// StepCreate runs the story creation step: invoke Claude to create a story file
// from a backlog entry, then verify post-conditions (file exists, status changed).
//
// Returns a StepResult for operational outcomes and an error for infrastructure failures.
func (p *Pipeline) StepCreate(ctx context.Context, key string) (StepResult, error) {
	start := time.Now()

	// Dry-run: skip subprocess invocation
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

	// Expand prompt template
	prompt, err := p.cfg.GetPrompt("create-story", key)
	if err != nil {
		return StepResult{}, fmt.Errorf("create story %s: %w", key, err)
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	// Build event handler: forward text to printer if verbose
	var handler claude.EventHandler
	if p.verbose && p.printer != nil {
		handler = func(event claude.Event) {
			if event.IsText() {
				p.printer.Text(event.Text)
			}
		}
	}

	// Run Claude
	exitCode, err := p.claude.ExecuteWithResult(timeoutCtx, prompt, handler)
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

// StepValidate runs the validation loop for a story file.
//
// It invokes Claude up to [maxValidationLoops] times using the create-story
// prompt (BMAD auto-detects validation when the file already exists).
// After each invocation, the story file's mtime is compared to its value
// before invocation. If the mtime is unchanged, the story has converged
// (no suggestions applied) and validation succeeds. If the mtime still
// changes on the final iteration, the story needs manual review.
//
// Returns a [StepResult] for operational outcomes (convergence or exhaustion)
// and an error for infrastructure failures (missing file, Claude error, etc.).
func (p *Pipeline) StepValidate(ctx context.Context, key string) (StepResult, error) {
	start := time.Now()

	if p.dryRun {
		msg := "dry-run: would validate story " + key
		if p.printer != nil {
			p.printer.Text(msg)
		}
		return StepResult{
			Name:    stepNameValidate,
			Success: true,
			Reason:  msg,
		}, nil
	}

	if p.status == nil {
		return StepResult{}, fmt.Errorf("stepValidate: status reader not configured")
	}
	storyDir, err := p.status.ResolveStoryLocation(p.projectDir)
	if err != nil {
		return StepResult{}, fmt.Errorf("stepValidate: resolve story location: %w", err)
	}
	storyPath := filepath.Join(storyDir, key+".md")

	prompt, err := p.cfg.GetPrompt("create-story", key)
	if err != nil {
		return StepResult{}, fmt.Errorf("stepValidate: failed to expand prompt: %w", err)
	}

	var handler claude.EventHandler
	if p.verbose && p.printer != nil {
		handler = func(event claude.Event) {
			if event.IsText() {
				p.printer.Text(event.Text)
			}
		}
	}

	for loop := 1; loop <= maxValidationLoops; loop++ {
		// Stat before invocation
		infoBefore, err := os.Stat(storyPath)
		if err != nil {
			return StepResult{}, fmt.Errorf("stepValidate: cannot stat story file: %w", err)
		}
		mtimeBefore := infoBefore.ModTime()

		// Invoke Claude (per-loop timeout, same as StepCreate)
		timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
		exitCode, err := p.claude.ExecuteWithResult(timeoutCtx, prompt, handler)
		ctxDone := timeoutCtx.Err()
		cancel()
		if err != nil {
			switch {
			case errors.Is(err, context.DeadlineExceeded) || errors.Is(ctxDone, context.DeadlineExceeded):
				return StepResult{
					Name:            stepNameValidate,
					Success:         false,
					Reason:          "validate story " + key + ": timed out",
					Duration:        time.Since(start),
					ValidationLoops: loop,
				}, nil
			case errors.Is(err, context.Canceled) || errors.Is(ctxDone, context.Canceled):
				return StepResult{
					Name:            stepNameValidate,
					Success:         false,
					Reason:          "validate story " + key + ": canceled",
					Duration:        time.Since(start),
					ValidationLoops: loop,
				}, nil
			default:
				return StepResult{}, fmt.Errorf("stepValidate: claude execution failed: %w", err)
			}
		}
		if exitCode != 0 {
			return StepResult{
				Name:            stepNameValidate,
				Success:         false,
				Reason:          fmt.Sprintf("validate story %s: claude exited with code %d", key, exitCode),
				Duration:        time.Since(start),
				ValidationLoops: loop,
			}, nil
		}

		// Stat after invocation
		infoAfter, err := os.Stat(storyPath)
		if err != nil {
			return StepResult{}, fmt.Errorf("stepValidate: cannot stat story file after invocation: %w", err)
		}
		mtimeAfter := infoAfter.ModTime()

		// Converged: mtime unchanged
		if mtimeAfter.Equal(mtimeBefore) {
			return StepResult{
				Name:            stepNameValidate,
				Success:         true,
				Duration:        time.Since(start),
				ValidationLoops: loop,
			}, nil
		}
	}

	// Exhausted all loops without convergence
	return StepResult{
		Name:            stepNameValidate,
		Success:         false,
		Reason:          "needs-review",
		Duration:        time.Since(start),
		ValidationLoops: maxValidationLoops,
	}, nil
}

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

	// 1. Read the story file
	content, err := os.ReadFile(storyPath)
	if err != nil {
		return StepResult{Name: stepNameSync}, fmt.Errorf("read story file %s: %w", storyPath, err)
	}

	// 2. Extract title
	title, err := beads.ExtractTitle(string(content))
	if err != nil {
		return StepResult{
			Name:     stepNameSync,
			Success:  false,
			Reason:   fmt.Sprintf("extract title: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	// 3. Extract acceptance criteria
	acs, err := beads.ExtractAcceptanceCriteria(string(content))
	if err != nil {
		return StepResult{
			Name:     stepNameSync,
			Success:  false,
			Reason:   fmt.Sprintf("extract ACs: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	// 4. Invoke bd create
	beadID, err := beadsExec.Create(ctx, key, title, acs, nil)
	if err != nil {
		return StepResult{
			Name:     stepNameSync,
			Success:  false,
			Reason:   fmt.Sprintf("bd create: %v", err),
			Duration: time.Since(start),
		}, nil
	}

	// 5. Append tracking comment
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
