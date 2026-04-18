package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"story-factory/internal/beads"
	"story-factory/internal/pipeline"
	"story-factory/internal/status"
)

// newQueueCommand creates the queue Cobra command.
//
// The queue command processes all backlog stories across all epics in sequence.
// Epics are processed in numeric order, and within each epic stories are
// processed in key order. Exit code 0 if all succeed or empty backlog,
// exit code 1 if any story failed.
func newQueueCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "queue",
		Short: "Process every unfinished story across all epics (anything not done)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Run preconditions (exit code 2 on failure)
			if err := app.RunPreconditions(); err != nil {
				return err
			}

			projectDir, err := app.ResolveProjectDir()
			if err != nil {
				return fmt.Errorf("failed to determine working directory: %w", err)
			}

			// Get the unfinished story list for display. The pipeline's batch
			// runner uses the same filter internally so the counts match.
			reader := status.NewReader(projectDir)
			stories, err := reader.UnfinishedStories()
			if err != nil {
				app.Printer.Text(fmt.Sprintf("Error reading sprint status: %s", err))
				return NewExitError(1)
			}

			storyKeys := make([]string, len(stories))
			for i, s := range stories {
				storyKeys[i] = s.Key
			}

			if len(stories) == 0 {
				app.Printer.Text("No unfinished stories in queue")
				return nil
			}

			app.Printer.QueueHeader(len(stories), storyKeys)

			// Construct executors with project working directory
			defaultExec, executors := buildCommandExecutors(app.Config, projectDir)

			// Construct beads executor with project working directory
			bdExecutor := &beads.DefaultExecutor{WorkingDir: projectDir}

			// Construct pipeline with all dependencies
			p := pipeline.NewPipeline(
				defaultExec,
				app.Config,
				projectDir,
				pipeline.WithLLMStepTimeout(pipeline.LLMStepTimeoutFromEnv()),
				pipeline.WithStatus(reader),
				pipeline.WithPrinter(app.Printer),
				pipeline.WithBeads(bdExecutor),
				pipeline.WithDryRun(app.DryRun),
				pipeline.WithVerbose(app.Verbose),
				pipeline.WithMode(app.Mode),
				pipeline.WithExecutors(executors),
			)

			// Execute queue
			result, err := p.RunQueue(cmd.Context())
			if err != nil {
				app.Printer.Text(fmt.Sprintf("Error: %s", err))
				return NewExitError(1)
			}

			// Convert pipeline results to output results for display
			outResults := mapStoryResults(result.Stories)
			counts := mapBatchCounts(result)
			app.Printer.BatchSummary(outResults, counts, result.Duration)

			if result.Failed > 0 {
				return NewExitError(1)
			}
			return nil
		},
	}
}
