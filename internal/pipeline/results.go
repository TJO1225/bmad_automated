package pipeline

import "time"

// StepResult represents the outcome of a single pipeline step execution.
type StepResult struct {
	Name            string
	Success         bool
	Reason          string
	Duration        time.Duration
	ValidationLoops int    // validate step only
	BeadID          string // sync step only
}

// StoryResult represents the outcome of processing a single story through the pipeline.
type StoryResult struct {
	Key      string
	Success  bool
	FailedAt string // step name, empty on success
	// Reason is the step failure message when Success is false; when Skipped is true,
	// it holds the story's current sprint status (e.g. "ready-for-dev").
	Reason          string
	Duration        time.Duration
	ValidationLoops int
	BeadID          string
	Skipped         bool // true if not in backlog (resumable)
}

// BatchResult represents the outcome of processing multiple stories.
type BatchResult struct {
	Stories   []StoryResult
	EpicNum   int // 0 for queue (all epics)
	Duration  time.Duration
	Created   int
	Validated int
	Synced    int
	Failed    int
	Skipped   int
}
