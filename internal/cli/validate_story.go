package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"story-factory/internal/claude"
	"story-factory/internal/pipeline"
	"story-factory/internal/status"
)

// NewValidateStoryCommand creates the validate-story Cobra command.
func NewValidateStoryCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "validate-story <story-key>",
		Short: "Validate a story file with auto-accept loop",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			storyKey := args[0]

			// Run preconditions
			if err := app.RunPreconditions(); err != nil {
				return err
			}

			// Resolve project directory
			projectDir, err := app.ResolveProjectDir()
			if err != nil {
				return fmt.Errorf("failed to determine working directory: %w", err)
			}

			// Verify story exists in sprint status
			entry, err := app.StatusReader.StoryByKey(storyKey)
			if err != nil {
				app.Printer.Text(fmt.Sprintf("Story not found: %s", storyKey))
				return NewExitError(1)
			}
			if entry.Status != status.StatusReadyForDev {
				app.Printer.Text(fmt.Sprintf("Story %s has status %q, expected %q", storyKey, entry.Status, status.StatusReadyForDev))
				return NewExitError(1)
			}

			storyDir, err := app.StatusReader.ResolveStoryLocation(projectDir)
			if err != nil {
				app.Printer.Text(fmt.Sprintf("Error: cannot resolve story location: %v", err))
				return NewExitError(1)
			}
			storyMarkdown := filepath.Join(storyDir, storyKey+".md")
			if _, err := os.Stat(storyMarkdown); err != nil {
				if os.IsNotExist(err) {
					app.Printer.Text(fmt.Sprintf("Story file not found: %s", storyMarkdown))
				} else {
					app.Printer.Text(fmt.Sprintf("Error accessing story file: %v", err))
				}
				return NewExitError(1)
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

			// Execute validation step
			result, err := p.StepValidate(cmd.Context(), storyKey)
			if err != nil {
				app.Printer.Text(fmt.Sprintf("Error: %s", err))
				return NewExitError(1)
			}

			if !result.Success {
				app.Printer.Text(fmt.Sprintf("Validation did not converge after %d loops: %s", result.ValidationLoops, result.Reason))
				return NewExitError(1)
			}

			app.Printer.Text(fmt.Sprintf("Story %s validated successfully (%d loop(s))", storyKey, result.ValidationLoops))
			return nil
		},
	}
}
