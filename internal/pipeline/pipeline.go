package pipeline

import (
	"context"
	"time"

	"story-factory/internal/status"
)

// stepFunc is the signature shared by all pipeline step methods.
type stepFunc func(context.Context, string) (StepResult, error)

// Run executes the full pipeline for a single story: create -> validate -> sync.
//
// Before running steps, it checks the story's sprint status. If the story is
// not in backlog, it returns a StoryResult with Skipped: true.
//
// Each step is retried once on operational failure (StepResult.Success == false),
// except sync, which is never retried (a retry could call bd create twice).
// Infrastructure failures (error return) are never retried and bubble up immediately.
// If a step fails after retry, the pipeline stops and returns the failure.
func (p *Pipeline) Run(ctx context.Context, key string) (StoryResult, error) {
	return p.runPipeline(ctx, key, p.StepCreate, p.StepValidate, p.stepSync)
}

// runPipeline implements the core pipeline logic with injectable step functions.
// This enables testing the orchestration (sequencing, retry, status checking)
// independently of step implementations.
func (p *Pipeline) runPipeline(ctx context.Context, key string, create, validate, sync stepFunc) (StoryResult, error) {
	start := time.Now()

	// Check story status — skip if not backlog
	entry, err := p.status.StoryByKey(key)
	if err != nil {
		return StoryResult{}, err
	}
	if entry.Status != status.StatusBacklog {
		return StoryResult{Key: key, Skipped: true, Reason: string(entry.Status)}, nil
	}

	if p.printer != nil {
		p.printer.Text("Starting pipeline for " + key)
	}

	// Step 1: Create
	if p.printer != nil {
		p.printer.StepStart(1, 3, "create")
	}
	createResult, err := p.runStep(ctx, key, create)
	if err != nil {
		return StoryResult{}, err
	}
	if p.printer != nil {
		p.printer.StepEnd(createResult.Duration, createResult.Success)
	}
	if !createResult.Success {
		return StoryResult{
			Key:      key,
			FailedAt: "create",
			Reason:   createResult.Reason,
			Duration: time.Since(start),
		}, nil
	}

	// Step 2: Validate
	if p.printer != nil {
		p.printer.StepStart(2, 3, "validate")
	}
	validateResult, err := p.runStep(ctx, key, validate)
	if err != nil {
		return StoryResult{}, err
	}
	if p.printer != nil {
		p.printer.StepEnd(validateResult.Duration, validateResult.Success)
	}
	if !validateResult.Success {
		return StoryResult{
			Key:      key,
			FailedAt: "validate",
			Reason:   validateResult.Reason,
			Duration: time.Since(start),
		}, nil
	}

	// Step 3: Sync
	if p.printer != nil {
		p.printer.StepStart(3, 3, "sync")
	}
	syncResult, err := p.runStep(ctx, key, sync)
	if err != nil {
		return StoryResult{}, err
	}
	if p.printer != nil {
		p.printer.StepEnd(syncResult.Duration, syncResult.Success)
	}
	if !syncResult.Success {
		return StoryResult{
			Key:      key,
			FailedAt: "sync",
			Reason:   syncResult.Reason,
			Duration: time.Since(start),
		}, nil
	}

	return StoryResult{
		Key:             key,
		Success:         true,
		Duration:        time.Since(start),
		ValidationLoops: validateResult.ValidationLoops,
		BeadID:          syncResult.BeadID,
	}, nil
}

// runStep executes a step function with retry-once semantics.
//
// On operational failure (StepResult.Success == false), the step is retried
// exactly once, except for the sync step, which is never retried.
// Infrastructure failures (error return) are never retried.
func (p *Pipeline) runStep(ctx context.Context, key string, stepFn stepFunc) (StepResult, error) {
	result, err := stepFn(ctx, key)
	if err != nil {
		return result, err
	}
	if result.Success {
		return result, nil
	}

	if result.Name == stepNameSync {
		return result, nil
	}

	// Operational failure — retry once
	if p.printer != nil {
		p.printer.Text("Retrying " + result.Name + "...")
	}
	result, err = stepFn(ctx, key)
	return result, err
}
