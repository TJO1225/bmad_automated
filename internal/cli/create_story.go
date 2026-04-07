package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"story-factory/internal/claude"
	"story-factory/internal/pipeline"
	"story-factory/internal/status"
)

// NewCreateStoryCommand creates the create-story Cobra command.
func NewCreateStoryCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "create-story <story-key>",
		Short: "Create a story file from a backlog entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			storyKey := strings.TrimSpace(args[0])
			if storyKey == "" {
				return fmt.Errorf("story key cannot be empty")
			}

			// Run preconditions
			if err := app.RunPreconditions(); err != nil {
				return err
			}

			// Resolve project directory
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

			// Construct pipeline
			p := pipeline.NewPipeline(
				executor,
				app.Config,
				projectDir,
				pipeline.WithStatus(status.NewReader(projectDir)),
				pipeline.WithPrinter(app.Printer),
				pipeline.WithDryRun(app.DryRun),
				pipeline.WithVerbose(app.Verbose),
			)

			// Execute step
			result, err := p.StepCreate(cmd.Context(), storyKey)
			if err != nil {
				app.Printer.Text(fmt.Sprintf("Error: %s", err))
				return NewExitError(1)
			}

			if !result.Success {
				app.Printer.Text(fmt.Sprintf("Failed: %s", result.Reason))
				return NewExitError(1)
			}

			app.Printer.Text(fmt.Sprintf("Story %s created successfully", storyKey))
			return nil
		},
	}
}
