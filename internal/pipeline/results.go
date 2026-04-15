package pipeline

import "time"

// StepResult represents the outcome of a single pipeline step execution.
type StepResult struct {
	Name     string
	Success  bool
	Reason   string
	Duration time.Duration
	// BeadID is populated by the sync-to-beads step only.
	BeadID string
	// PRURL is populated by the open-pr step only (Phase 2).
	PRURL string
}

// StoryResult represents the outcome of processing a single story through the pipeline.
type StoryResult struct {
	// Key is the story key from sprint-status.yaml.
	Key string
	// Success is true when every step in the mode's sequence returned Success.
	Success bool
	// FailedAt is the name of the step that failed; empty on success.
	FailedAt string
	// Reason is the step failure message when Success is false; when Skipped is true,
	// it holds the story's current sprint status (e.g. "done").
	Reason string
	// Duration is the total wall-clock time for the pipeline run.
	Duration time.Duration
	// BeadID is populated when the pipeline ran sync-to-beads successfully.
	BeadID string
	// PRURL is populated when the pipeline ran open-pr successfully (Phase 2).
	PRURL string
	// NeedsReview is true when code-review flipped the story back to in-progress
	// because findings remain. The per-story exit is non-success but is surfaced
	// separately in the summary so the user knows to re-run after addressing.
	NeedsReview bool
	// Skipped is true when the pipeline was short-circuited (e.g. story already done).
	Skipped bool
	// StepsExecuted lists the step names that actually ran, in order. Used by
	// batch-level counters to attribute success/failure to specific steps.
	StepsExecuted []string
}

// BatchResult represents the outcome of processing multiple stories.
type BatchResult struct {
	Stories  []StoryResult
	EpicNum  int // 0 for queue (all epics)
	Duration time.Duration
	// StepCounts maps step name to the number of stories that completed that
	// step successfully. The set of keys depends on which mode ran; callers
	// that need totals should iterate the map rather than looking up fixed names.
	StepCounts map[string]int
	// Failed counts stories whose pipeline ended in non-skipped failure.
	Failed int
	// Skipped counts stories short-circuited before running any step (e.g. already done).
	Skipped int
	// NeedsReview counts stories that failed specifically at code-review because
	// findings remain — a subset of Failed, surfaced separately.
	NeedsReview int
}
