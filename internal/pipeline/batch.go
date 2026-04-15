package pipeline

import (
	"context"
	"fmt"
	"time"

	"story-factory/internal/status"
)

// runFunc is the signature for running a single story through the pipeline.
type runFunc func(context.Context, string) (StoryResult, error)

// RunQueue processes all backlog stories across all epics in sequence.
//
// Stories are sorted by epic number then story number (guaranteed by
// [status.Reader.BacklogStories]). Before each story, sprint-status.yaml
// is re-read to detect status changes made by BMAD between operations.
// Stories that are no longer in backlog on re-read are skipped (resumable
// batch support). A single story failure does not abort the batch.
func (p *Pipeline) RunQueue(ctx context.Context) (BatchResult, error) {
	return p.runQueue(ctx, p.Run)
}

// runQueue implements the batch logic with an injectable run function for testing.
func (p *Pipeline) runQueue(ctx context.Context, run runFunc) (BatchResult, error) {
	start := time.Now()

	stories, err := p.status.UnfinishedStories()
	if err != nil {
		return BatchResult{}, err
	}

	if len(stories) == 0 {
		if p.printer != nil {
			p.printer.Text("No unfinished stories found")
		}
		return BatchResult{Duration: time.Since(start), StepCounts: map[string]int{}}, nil
	}

	if p.printer != nil {
		p.printer.Text(fmt.Sprintf("Processing queue: %d unfinished stories", len(stories)))
	}

	result := p.runBatch(ctx, stories, run)
	result.Duration = time.Since(start)
	return result, nil
}

// RunEpic processes backlog stories for a single epic from sprint-status.yaml, in story-number order.
//
// Stories are listed via [status.Reader.StoriesForEpic] and filtered to backlog-only.
// Before each story, a fresh [status.Reader] re-reads sprint-status.yaml (same as [Pipeline.RunQueue]).
// Stories that are no longer in backlog on re-read are skipped (resumable batch support).
func (p *Pipeline) RunEpic(ctx context.Context, epicNum int) (BatchResult, error) {
	return p.runEpic(ctx, epicNum, p.Run)
}

// runEpic implements the epic batch logic with an injectable run function.
func (p *Pipeline) runEpic(ctx context.Context, epicNum int, run runFunc) (BatchResult, error) {
	start := time.Now()

	allStories, err := p.status.StoriesForEpic(epicNum)
	if err != nil {
		return BatchResult{}, err
	}

	if len(allStories) == 0 {
		if p.printer != nil {
			p.printer.Text(fmt.Sprintf("No stories found for epic %d", epicNum))
		}
		return BatchResult{EpicNum: epicNum, Duration: time.Since(start), StepCounts: map[string]int{}}, nil
	}

	var pendingEntries []status.Entry
	for _, e := range allStories {
		if e.Status != status.StatusDone {
			pendingEntries = append(pendingEntries, e)
		}
	}

	if len(pendingEntries) == 0 {
		if p.printer != nil {
			p.printer.Text(fmt.Sprintf("No unfinished stories found for epic %d", epicNum))
		}
		return BatchResult{EpicNum: epicNum, Duration: time.Since(start), StepCounts: map[string]int{}}, nil
	}

	if p.printer != nil {
		p.printer.Text(fmt.Sprintf("Processing epic %d: %d unfinished stories", epicNum, len(pendingEntries)))
	}

	result := p.runBatch(ctx, pendingEntries, run)
	result.EpicNum = epicNum
	result.Duration = time.Since(start)
	return result, nil
}

// runBatch processes a backlog story list for [RunQueue] and [RunEpic].
//
// Before each story, a fresh [status.Reader] re-reads sprint-status.yaml so keys that left
// backlog between iterations are skipped without invoking run (needed when run is a mock).
// Infrastructure errors from run are recorded as failures without aborting the batch.
func (p *Pipeline) runBatch(ctx context.Context, stories []status.Entry, run runFunc) BatchResult {
	result := BatchResult{StepCounts: map[string]int{}}

	for i, entry := range stories {
		if p.printer != nil {
			p.printer.Text(fmt.Sprintf("[%d/%d] %s", i+1, len(stories), entry.Key))
		}

		// Re-read fresh status before each story. Only done stories are
		// skipped at this layer — ready-for-dev / in-progress / review
		// stories still flow into Run, which applies per-step resume logic.
		freshReader := status.NewReader(p.projectDir)
		freshEntry, err := freshReader.StoryByKey(entry.Key)
		if err != nil {
			result.Stories = append(result.Stories, StoryResult{
				Key:      entry.Key,
				FailedAt: "infrastructure",
				Reason:   err.Error(),
			})
			result.Failed++
			continue
		}
		if freshEntry.Status == status.StatusDone {
			result.Stories = append(result.Stories, StoryResult{Key: entry.Key, Skipped: true, Reason: string(status.StatusDone)})
			result.Skipped++
			continue
		}

		storyResult, err := run(ctx, entry.Key)
		if err != nil {
			result.Stories = append(result.Stories, StoryResult{
				Key:      entry.Key,
				FailedAt: "infrastructure",
				Reason:   err.Error(),
			})
			result.Failed++
			continue
		}

		result.Stories = append(result.Stories, storyResult)
		switch {
		case storyResult.Skipped:
			result.Skipped++
		case storyResult.Success:
			for _, stepName := range storyResult.StepsExecuted {
				result.StepCounts[stepName]++
			}
		default:
			result.Failed++
			if storyResult.NeedsReview {
				result.NeedsReview++
			}
			// Count steps that succeeded before the failing step.
			// StepsExecuted includes the failed step at the end, so drop it.
			for i, stepName := range storyResult.StepsExecuted {
				if i == len(storyResult.StepsExecuted)-1 {
					break
				}
				result.StepCounts[stepName]++
			}
		}
	}

	return result
}
