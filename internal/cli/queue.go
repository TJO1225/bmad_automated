package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"story-factory/internal/beads"
	"story-factory/internal/claude"
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
		Short: "Process all backlog stories across all epics",
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

			// Get backlog story list for display
			reader := status.NewReader(projectDir)
			stories, err := reader.BacklogStories()
			if err != nil {
				app.Printer.Text(fmt.Sprintf("Error reading sprint status: %s", err))
				return NewExitError(1)
			}

			storyKeys := make([]string, len(stories))
			for i, s := range stories {
				storyKeys[i] = s.Key
			}

			if len(stories) == 0 {
				app.Printer.Text("No backlog stories in queue")
				return nil
			}

			app.Printer.QueueHeader(len(stories), storyKeys)

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
