// Package status provides functionality for reading sprint status YAML files.
//
// The sprint-status.yaml file tracks the development status of stories throughout
// their lifecycle. Each story progresses through statuses: backlog -> ready-for-dev ->
// in-progress -> review -> done.
//
// Key types:
//   - [Status] - Story development status enum with validation
//   - [EntryType] - Classification for development_status entries (epic, story, retrospective)
//   - [Entry] - A single parsed entry from the development_status map
//   - [Reader] - Reads and queries sprint status from YAML files
//
// The package uses yaml.v3's Node API to preserve entry ordering when parsing
// the status file.
package status

import "errors"

// ErrStoryNotFound is returned when a story key is not found in the sprint status file.
var ErrStoryNotFound = errors.New("story not found")

// Status represents a story's development status in the workflow lifecycle.
//
// A story progresses through statuses as it moves through development:
// backlog -> ready-for-dev -> in-progress -> review -> done.
//
// The status determines which workflow command is executed when running a story.
type Status string

// Status constants define the valid development statuses in the story lifecycle.
const (
	// StatusBacklog indicates a story that has not been started.
	// Stories in backlog trigger the create-story workflow.
	StatusBacklog Status = "backlog"

	// StatusReadyForDev indicates a story is ready for development.
	// Stories in ready-for-dev trigger the dev-story workflow.
	StatusReadyForDev Status = "ready-for-dev"

	// StatusInProgress indicates a story is actively being developed.
	// Stories in progress trigger the dev-story workflow to continue work.
	StatusInProgress Status = "in-progress"

	// StatusReview indicates a story is ready for code review.
	// Stories in review trigger the code-review workflow.
	StatusReview Status = "review"

	// StatusDone indicates a story has completed all workflow steps.
	// Stories marked done are skipped in queue and epic operations.
	StatusDone Status = "done"
)

// IsProcessable reports whether a story with this status can advance
// through the story-factory pipeline. True for backlog, ready-for-dev,
// in-progress, and review. Done stories are already complete. Anything
// else (project-custom statuses such as "deferred-post-mvp") is treated
// as a hard skip — the pipeline will not try to run, branch, or open PRs
// for those stories.
func (s Status) IsProcessable() bool {
	switch s {
	case StatusBacklog, StatusReadyForDev, StatusInProgress, StatusReview:
		return true
	default:
		return false
	}
}

// IsValid reports whether the status is one of the known valid status values.
// It returns true for backlog, ready-for-dev, in-progress, review, and done.
func (s Status) IsValid() bool {
	switch s {
	case StatusBacklog, StatusReadyForDev, StatusInProgress, StatusReview, StatusDone:
		return true
	default:
		return false
	}
}

// EntryType classifies a development_status entry.
type EntryType int

const (
	// EntryTypeEpic represents an epic entry (key pattern: epic-N).
	EntryTypeEpic EntryType = iota

	// EntryTypeStory represents a story entry (key pattern: N-M-slug).
	EntryTypeStory

	// EntryTypeRetrospective represents a retrospective entry (key pattern: epic-N-retrospective).
	EntryTypeRetrospective
)

// String returns a human-readable representation of the EntryType.
func (t EntryType) String() string {
	switch t {
	case EntryTypeEpic:
		return "epic"
	case EntryTypeStory:
		return "story"
	case EntryTypeRetrospective:
		return "retrospective"
	default:
		return "unknown"
	}
}

// Entry represents a single parsed entry from the development_status map.
type Entry struct {
	Key      string    // Raw key, e.g. "1-2-sprint-status-yaml-parser"
	Status   Status    // Parsed status value
	Type     EntryType // Classified type
	EpicNum  int       // Epic number (all types have this)
	StoryNum int       // Story number (only EntryTypeStory, 0 for others)
	Slug     string    // Slug portion (only EntryTypeStory, empty for others)
}
