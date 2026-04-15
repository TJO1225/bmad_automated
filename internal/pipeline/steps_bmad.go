package pipeline

import (
	"context"
	"errors"
	"fmt"
	"time"

	"story-factory/internal/claude"
	"story-factory/internal/status"
)

// StepDevStory invokes BMAD's /bmad-dev-story skill on a story file and
// verifies the sprint-status flipped to "review" on success.
//
// Preconditions: the story file must already exist at
// {storyDir}/{key}.md (produced by create-story) and the sprint status must
// be either "ready-for-dev" (fresh) or "in-progress" (resumed after a
// previous partial run). Any other status returns an operational failure.
//
// Postcondition: sprint status advances to "review". If the status is still
// "ready-for-dev" or "in-progress" after Claude exits, the step treats it as
// an operational failure so the pipeline can retry.
//
// Returns an error for infrastructure failures (missing status reader, Claude
// subprocess error) and a StepResult for operational outcomes.
func (p *Pipeline) StepDevStory(ctx context.Context, key string) (StepResult, error) {
	start := time.Now()

	if p.dryRun {
		msg := "dry-run: would dev story " + key
		if p.printer != nil {
			p.printer.Text(msg)
		}
		return StepResult{
			Name:    stepNameDevStory,
			Success: true,
			Reason:  msg,
		}, nil
	}

	if p.status == nil {
		return StepResult{}, fmt.Errorf("dev story %s: pipeline has no status reader", key)
	}

	// Pre-condition + resume logic: reconcile with current sprint status.
	//   - ready-for-dev, in-progress → run (normal path / BMAD resume within skill)
	//   - review, done               → skip success (prior run already completed dev)
	//   - backlog                    → fail (create-story didn't run)
	preReader := status.NewReader(p.projectDir)
	preEntry, err := preReader.StoryByKey(key)
	if err != nil {
		return StepResult{}, fmt.Errorf("dev story %s: %w", key, err)
	}
	switch preEntry.Status {
	case status.StatusReadyForDev, status.StatusInProgress:
		// fall through to invoke Claude
	case status.StatusReview, status.StatusDone:
		msg := fmt.Sprintf("dev-story already complete (status: %s)", preEntry.Status)
		if p.printer != nil {
			p.printer.Text(msg)
		}
		return StepResult{
			Name:     stepNameDevStory,
			Success:  true,
			Reason:   msg,
			Duration: time.Since(start),
		}, nil
	default:
		return StepResult{
			Name:     stepNameDevStory,
			Success:  false,
			Reason:   fmt.Sprintf("dev story %s: status is %q, expected ready-for-dev or in-progress (run create-story first)", key, preEntry.Status),
			Duration: time.Since(start),
		}, nil
	}

	prompt, err := p.cfg.GetPrompt("dev-story", key)
	if err != nil {
		return StepResult{}, fmt.Errorf("dev story %s: %w", key, err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	handler := p.verboseHandler()
	exitCode, err := p.claude.ExecuteWithResult(timeoutCtx, prompt, handler)
	if err != nil {
		if reason := classifyContextErr(err, timeoutCtx, "dev story "+key); reason != "" {
			return StepResult{
				Name:     stepNameDevStory,
				Success:  false,
				Reason:   reason,
				Duration: time.Since(start),
			}, nil
		}
		return StepResult{}, err
	}
	if exitCode != 0 {
		return StepResult{
			Name:     stepNameDevStory,
			Success:  false,
			Reason:   fmt.Sprintf("dev story %s: claude exited with code %d", key, exitCode),
			Duration: time.Since(start),
		}, nil
	}

	// Post-condition: status advanced to review.
	postReader := status.NewReader(p.projectDir)
	postEntry, err := postReader.StoryByKey(key)
	if err != nil {
		return StepResult{}, fmt.Errorf("dev story %s: read sprint status: %w", key, err)
	}
	if postEntry.Status != status.StatusReview {
		return StepResult{
			Name:     stepNameDevStory,
			Success:  false,
			Reason:   fmt.Sprintf("dev story %s: status is %q after dev-story, expected review", key, postEntry.Status),
			Duration: time.Since(start),
		}, nil
	}

	return StepResult{
		Name:     stepNameDevStory,
		Success:  true,
		Duration: time.Since(start),
	}, nil
}

// StepCodeReview invokes BMAD's /bmad-code-review skill on a reviewable story
// and interprets the resulting sprint-status transition.
//
// Preconditions: sprint status must be "review" (produced by dev-story).
//
// Postcondition interpretation:
//   - status becomes "done" → success
//   - status becomes "in-progress" → operational failure with
//     Reason=[codeReviewNeedsReviewReason]. The pipeline driver maps this
//     onto [StoryResult.NeedsReview] so the batch summary can surface it
//     distinctly from other failures.
//   - status unchanged from "review" or any other value → operational failure.
//
// The code-review step is non-retryable: re-running against the same diff
// reproduces the same findings.
func (p *Pipeline) StepCodeReview(ctx context.Context, key string) (StepResult, error) {
	start := time.Now()

	if p.dryRun {
		msg := "dry-run: would code-review story " + key
		if p.printer != nil {
			p.printer.Text(msg)
		}
		return StepResult{
			Name:    stepNameCodeReview,
			Success: true,
			Reason:  msg,
		}, nil
	}

	if p.status == nil {
		return StepResult{}, fmt.Errorf("code-review %s: pipeline has no status reader", key)
	}

	preReader := status.NewReader(p.projectDir)
	preEntry, err := preReader.StoryByKey(key)
	if err != nil {
		return StepResult{}, fmt.Errorf("code-review %s: %w", key, err)
	}
	switch preEntry.Status {
	case status.StatusReview:
		// fall through to invoke Claude
	case status.StatusDone:
		msg := "code-review already complete (status: done)"
		if p.printer != nil {
			p.printer.Text(msg)
		}
		return StepResult{
			Name:     stepNameCodeReview,
			Success:  true,
			Reason:   msg,
			Duration: time.Since(start),
		}, nil
	default:
		return StepResult{
			Name:     stepNameCodeReview,
			Success:  false,
			Reason:   fmt.Sprintf("code-review %s: status is %q, expected review", key, preEntry.Status),
			Duration: time.Since(start),
		}, nil
	}

	prompt, err := p.cfg.GetPrompt("code-review", key)
	if err != nil {
		return StepResult{}, fmt.Errorf("code-review %s: %w", key, err)
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, DefaultTimeout)
	defer cancel()

	handler := p.verboseHandler()
	exitCode, err := p.claude.ExecuteWithResult(timeoutCtx, prompt, handler)
	if err != nil {
		if reason := classifyContextErr(err, timeoutCtx, "code-review "+key); reason != "" {
			return StepResult{
				Name:     stepNameCodeReview,
				Success:  false,
				Reason:   reason,
				Duration: time.Since(start),
			}, nil
		}
		return StepResult{}, err
	}
	if exitCode != 0 {
		return StepResult{
			Name:     stepNameCodeReview,
			Success:  false,
			Reason:   fmt.Sprintf("code-review %s: claude exited with code %d", key, exitCode),
			Duration: time.Since(start),
		}, nil
	}

	postReader := status.NewReader(p.projectDir)
	postEntry, err := postReader.StoryByKey(key)
	if err != nil {
		return StepResult{}, fmt.Errorf("code-review %s: read sprint status: %w", key, err)
	}

	switch postEntry.Status {
	case status.StatusDone:
		return StepResult{
			Name:     stepNameCodeReview,
			Success:  true,
			Duration: time.Since(start),
		}, nil
	case status.StatusInProgress:
		return StepResult{
			Name:     stepNameCodeReview,
			Success:  false,
			Reason:   codeReviewNeedsReviewReason,
			Duration: time.Since(start),
		}, nil
	default:
		return StepResult{
			Name:     stepNameCodeReview,
			Success:  false,
			Reason:   fmt.Sprintf("code-review %s: status is %q after code-review, expected done or in-progress", key, postEntry.Status),
			Duration: time.Since(start),
		}, nil
	}
}

// verboseHandler returns an event handler that forwards Claude text events to
// the pipeline's printer when verbose mode is enabled. Returns nil if either
// verbose is off or no printer is configured, in which case the executor runs
// silently.
func (p *Pipeline) verboseHandler() claude.EventHandler {
	if !p.verbose || p.printer == nil {
		return nil
	}
	return func(event claude.Event) {
		if event.IsText() {
			p.printer.Text(event.Text)
		}
	}
}

// classifyContextErr returns a StepResult.Reason string if the error is a
// context deadline or cancellation, otherwise "". Callers use the empty
// return as a signal to propagate the original infrastructure error.
func classifyContextErr(err error, timeoutCtx context.Context, op string) string {
	switch {
	case errors.Is(err, context.DeadlineExceeded) || errors.Is(timeoutCtx.Err(), context.DeadlineExceeded):
		return op + ": timed out"
	case errors.Is(err, context.Canceled) || errors.Is(timeoutCtx.Err(), context.Canceled):
		return op + ": canceled"
	default:
		return ""
	}
}
