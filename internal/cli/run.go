package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"story-factory/internal/beads"
	"story-factory/internal/pipeline"
	"story-factory/internal/status"
)

// newRunCommand creates the run Cobra command.
//
// The run command executes the configured pipeline mode's full step sequence
// for a single story key. The step list is mode-dependent; see --mode.
func newRunCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "run <story-key>",
		Short: "Run the full pipeline for a story (steps depend on --mode)",
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
				pipeline.WithStatus(status.NewReader(projectDir)),
				pipeline.WithPrinter(app.Printer),
				pipeline.WithBeads(bdExecutor),
				pipeline.WithDryRun(app.DryRun),
				pipeline.WithVerbose(app.Verbose),
				pipeline.WithMode(app.Mode),
				pipeline.WithExecutors(executors),
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
				if result.NeedsReview {
					app.Printer.Text(fmt.Sprintf("Story %s needs review — re-run after addressing findings", storyKey))
				}
				app.Printer.CycleFailed(storyKey, result.FailedAt, result.Duration)
				return NewExitError(1)
			}

			details := fmt.Sprintf("steps: %v", result.StepsExecuted)
			if result.BeadID != "" {
				details += fmt.Sprintf(", bead: %s", result.BeadID)
			}
			if result.PRURL != "" {
				details += fmt.Sprintf(", pr: %s", result.PRURL)
			}
			app.Printer.Text(fmt.Sprintf("Story %s processed successfully (%s)", storyKey, details))
			return nil
		},
	}
}
