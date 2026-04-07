package cli

import (
	"fmt"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"story-factory/internal/beads"
	"story-factory/internal/claude"
	"story-factory/internal/output"
	"story-factory/internal/pipeline"
	"story-factory/internal/status"
)

// newEpicCommand creates the epic Cobra command.
//
// The epic command runs the full pipeline (create -> validate -> sync) for all
// backlog stories in an epic, processing them sequentially in key order.
// Exit code 0 if all succeed or no backlog stories, exit code 1 if any failed.
func newEpicCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "epic <epic-number>",
		Short: "Run the full pipeline for all backlog stories in an epic",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			epicNum, err := strconv.Atoi(args[0])
			if err != nil || epicNum < 1 {
				return fmt.Errorf("invalid epic number: %s (must be a positive integer)", args[0])
			}

			// Run preconditions (exit code 2 on failure)
			if err := app.RunPreconditions(); err != nil {
				return err
			}

			projectDir, err := app.ResolveProjectDir()
			if err != nil {
				return fmt.Errorf("failed to determine working directory: %w", err)
			}

			// Get backlog story list for display
			reader := status.NewReader(projectDir)
			allStories, err := reader.StoriesForEpic(epicNum)
			if err != nil {
				app.Printer.Text(fmt.Sprintf("Error reading sprint status: %s", err))
				return NewExitError(1)
			}

			var storyKeys []string
			for _, s := range allStories {
				if s.Status == status.StatusBacklog {
					storyKeys = append(storyKeys, s.Key)
				}
			}

			if len(storyKeys) == 0 {
				app.Printer.Text("No backlog stories for epic " + args[0])
				return nil
			}

			app.Printer.QueueHeader(len(storyKeys), storyKeys)

			// Construct executor with project working directory
			executor := claude.NewExecutor(claude.ExecutorConfig{
				BinaryPath:   app.Config.Claude.BinaryPath,
				OutputFormat: app.Config.Claude.OutputFormat,
				WorkingDir:   projectDir,
				GracePeriod:  5 * time.Second,
			})

			// Construct beads executor with project working directory
			bdExecutor := &beads.DefaultExecutor{WorkingDir: projectDir}

			// Construct pipeline with all dependencies
			p := pipeline.NewPipeline(
				executor,
				app.Config,
				projectDir,
				pipeline.WithStatus(reader),
				pipeline.WithPrinter(app.Printer),
				pipeline.WithBeads(bdExecutor),
				pipeline.WithDryRun(app.DryRun),
				pipeline.WithVerbose(app.Verbose),
			)

			// Execute epic batch
			result, err := p.RunEpic(cmd.Context(), epicNum)
			if err != nil {
				app.Printer.Text(fmt.Sprintf("Error: %s", err))
				return NewExitError(1)
			}

			// Convert pipeline results to output results for display
			outResults := mapStoryResults(result.Stories)
			counts := mapBatchCounts(result)
			app.Printer.QueueSummary(outResults, counts, result.Duration)

			if result.Failed > 0 {
				return NewExitError(1)
			}
			return nil
		},
	}
}

// mapStoryResults converts pipeline story results to output story results.
func mapStoryResults(stories []pipeline.StoryResult) []output.StoryResult {
	out := make([]output.StoryResult, len(stories))
	for i, sr := range stories {
		out[i] = output.StoryResult{
			Key:             sr.Key,
			Success:         sr.Success,
			Duration:        sr.Duration,
			FailedAt:        sr.FailedAt,
			Reason:          sr.Reason,
			Skipped:         sr.Skipped,
			ValidationLoops: sr.ValidationLoops,
			BeadID:          sr.BeadID,
		}
	}
	return out
}

// mapBatchCounts converts a pipeline BatchResult's counts to output BatchCounts.
func mapBatchCounts(result pipeline.BatchResult) output.BatchCounts {
	return output.BatchCounts{
		Created:   result.Created,
		Validated: result.Validated,
		Synced:    result.Synced,
		Failed:    result.Failed,
		Skipped:   result.Skipped,
	}
}
