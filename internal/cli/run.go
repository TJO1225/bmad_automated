package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"story-factory/internal/claude"
	"story-factory/internal/pipeline"
	"story-factory/internal/status"
)

// newRunCommand creates the run Cobra command.
//
// The run command executes the full pipeline (create -> validate -> sync) for a
// single story key.
func newRunCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "run <story-key>",
		Short: "Run the full create-validate-sync pipeline for a story",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			storyKey := args[0]

			// Run preconditions (exit code 2 on failure)
			if err := app.RunPreconditions(); err != nil {
				return err
			}

			projectDir, err := app.ResolveProjectDir()
			if err != nil {
				return fmt.Errorf("failed to determine working directory: %w", err)
			}

			// Construct executor with project working directory
			executor := claude.NewExecutor(claude.ExecutorConfig{
				BinaryPath:   app.Config.Claude.BinaryPath,
				OutputFormat: app.Config.Claude.OutputFormat,
				WorkingDir:   projectDir,
				GracePeriod:  5 * time.Second,
			})

			// Construct pipeline with all dependencies
			p := pipeline.NewPipeline(
				executor,
				app.Config,
				projectDir,
				pipeline.WithStatus(status.NewReader(projectDir)),
				pipeline.WithPrinter(app.Printer),
				pipeline.WithBeads(app.BeadsExecutor),
				pipeline.WithDryRun(app.DryRun),
				pipeline.WithVerbose(app.Verbose),
			)

			// Execute full pipeline
			result, err := p.Run(cmd.Context(), storyKey)
			if err != nil {
				app.Printer.Text(fmt.Sprintf("Error: %s", err))
				return NewExitError(1)
			}

			if result.Skipped {
				msg := result.Reason
				if msg == "" {
					msg = "not backlog"
				}
				app.Printer.Text(fmt.Sprintf("Story %s skipped (status: %s)", storyKey, msg))
				return nil
			}

			if !result.Success {
				if result.Reason != "" {
					app.Printer.Text(fmt.Sprintf("Reason: %s", result.Reason))
				}
				app.Printer.CycleFailed(storyKey, result.FailedAt, result.Duration)
				return NewExitError(1)
			}

			app.Printer.Text(fmt.Sprintf("Story %s processed successfully (bead: %s, validation loops: %d)",
				storyKey, result.BeadID, result.ValidationLoops))
			return nil
		},
	}
}
