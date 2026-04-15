package pipeline

import (
	"context"
	"fmt"
	"time"

	"story-factory/internal/status"
)

// stepFunc is the signature shared by all pipeline step methods.
type stepFunc func(context.Context, string) (StepResult, error)

// namedStep pairs a step name with its executable function so the pipeline
// can report which step ran, apply per-step retry policy, and resolve steps
// from config by name.
type namedStep struct {
	Name string
	Fn   stepFunc
}

// nonRetryableSteps lists step names that must not be retried on operational
// failure. Retrying these would cause duplicate side effects (e.g. two
// bd-create calls, two commits, two PRs). code-review is also non-retryable
// because the same diff reproduces the same findings.
var nonRetryableSteps = map[string]struct{}{
	stepNameSync:         {},
	stepNameCommitBranch: {},
	stepNameOpenPR:       {},
	stepNameCodeReview:   {},
}

// codeReviewNeedsReviewReason is the sentinel [StepResult.Reason] returned by
// StepCodeReview when the story was flipped back to in-progress. The pipeline
// driver maps this onto [StoryResult.NeedsReview] rather than treating it as
// an opaque failure.
const codeReviewNeedsReviewReason = "needs-review"

// stepRegistry returns the mapping of step names to Pipeline method functions.
// Unknown names from a mode configuration produce a startup error in [Run].
func (p *Pipeline) stepRegistry() map[string]stepFunc {
	return map[string]stepFunc{
		stepNameCreate:       p.StepCreate,
		stepNameDevStory:     p.StepDevStory,
		stepNameCodeReview:   p.StepCodeReview,
		stepNameSync:         p.stepSync,
		stepNameCommitBranch: p.StepCommitBranch,
		stepNameOpenPR:       p.StepOpenPR,
	}
}

// Run executes the configured pipeline mode's step sequence for a single story.
//
// The mode is looked up from [Pipeline.cfg].Modes; each named step is resolved
// against [Pipeline.stepRegistry]. Before running any steps, the story's sprint
// status is checked: stories already at "done" are skipped and returned with
// [StoryResult.Skipped] set. Each step is retried at most once on operational
// failure, except for steps in [nonRetryableSteps]. Infrastructure errors
// (non-nil err) are never retried and bubble up immediately.
func (p *Pipeline) Run(ctx context.Context, key string) (StoryResult, error) {
	mode := p.mode
	if mode == "" {
		mode = "bmad"
	}
	stepNames, err := p.cfg.GetModeSteps(mode)
	if err != nil {
		return StoryResult{}, err
	}
	registry := p.stepRegistry()
	steps := make([]namedStep, 0, len(stepNames))
	for _, name := range stepNames {
		fn, ok := registry[name]
		if !ok {
			return StoryResult{}, fmt.Errorf("mode %q references unknown step %q", mode, name)
		}
		steps = append(steps, namedStep{Name: name, Fn: fn})
	}
	return p.runPipeline(ctx, key, steps)
}

// runPipeline executes the given step slice sequentially, honoring retry
// policy and capturing per-step outcomes on the resulting [StoryResult].
//
// Exported tests inject synthetic step slices to verify sequencing, retry,
// and failure-propagation independent of real step implementations.
func (p *Pipeline) runPipeline(ctx context.Context, key string, steps []namedStep) (StoryResult, error) {
	start := time.Now()

	entry, err := p.status.StoryByKey(key)
	if err != nil {
		return StoryResult{}, err
	}
	if entry.Status == status.StatusDone {
		return StoryResult{Key: key, Skipped: true, Reason: string(entry.Status)}, nil
	}

	if p.printer != nil {
		p.printer.Text("Starting pipeline for " + key)
	}

	executed := make([]string, 0, len(steps))
	var result StoryResult
	result.Key = key

	total := len(steps)
	for i, step := range steps {
		if p.printer != nil {
			p.printer.StepStart(i+1, total, step.Name)
		}
		stepResult, err := p.runStep(ctx, key, step)
		if err != nil {
			return StoryResult{}, err
		}
		if p.printer != nil {
			p.printer.StepEnd(stepResult.Duration, stepResult.Success)
		}
		executed = append(executed, step.Name)

		// Capture per-step data on the StoryResult for summary display.
		if stepResult.BeadID != "" {
			result.BeadID = stepResult.BeadID
		}
		if stepResult.PRURL != "" {
			result.PRURL = stepResult.PRURL
		}

		if !stepResult.Success {
			result.FailedAt = step.Name
			result.Reason = stepResult.Reason
			result.Duration = time.Since(start)
			result.StepsExecuted = executed
			if step.Name == "code-review" && stepResult.Reason == codeReviewNeedsReviewReason {
				result.NeedsReview = true
			}
			return result, nil
		}
	}

	result.Success = true
	result.Duration = time.Since(start)
	result.StepsExecuted = executed
	return result, nil
}

// runStep executes a step with retry-once semantics.
//
// Operational failures (StepResult.Success == false) are retried exactly once
// unless the step name is in [nonRetryableSteps]. Infrastructure errors
// (non-nil err) are never retried.
func (p *Pipeline) runStep(ctx context.Context, key string, step namedStep) (StepResult, error) {
	result, err := step.Fn(ctx, key)
	if err != nil {
		return result, err
	}
	if result.Success {
		return result, nil
	}

	if _, nonRetryable := nonRetryableSteps[step.Name]; nonRetryable {
		return result, nil
	}

	if p.printer != nil {
		p.printer.Text("Retrying " + step.Name + "...")
	}
	result, err = step.Fn(ctx, key)
	return result, err
}
