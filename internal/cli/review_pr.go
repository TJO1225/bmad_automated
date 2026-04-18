package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"story-factory/internal/pipeline"
	"story-factory/internal/status"
)

// newReviewPRCommand creates the review-pr Cobra command.
//
// The review-pr command runs only the review-pr step for a single story.
// It looks up the PR for story/<key>, executes a code review using the
// configured backend (default: gemini), and auto-merges on success.
//
// This command can be used standalone (e.g., to review a PR created
// manually or by a prior pipeline run) or as part of the full pipeline.
func newReviewPRCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "review-pr <story-key>",
		Short: "Review and auto-merge a PR for a story (uses a different LLM backend)",
		Long: `Review a pull request for story/<key> using a code review backend
(default: gemini) and auto-merge if the review passes.

The PR must already exist (created by open-pr or manually). The review
backend is configured in workflows.yaml under review-pr.backend.`,
		Args: cobra.ExactArgs(1),
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

			// Construct pipeline
			p := pipeline.NewPipeline(
				defaultExec,
				app.Config,
				projectDir,
				pipeline.WithLLMStepTimeout(pipeline.LLMStepTimeoutFromEnv()),
				pipeline.WithStatus(status.NewReader(projectDir)),
				pipeline.WithPrinter(app.Printer),
				pipeline.WithDryRun(app.DryRun),
				pipeline.WithVerbose(app.Verbose),
				pipeline.WithMode(app.Mode),
				pipeline.WithExecutors(executors),
			)

			// Execute review-pr step only
			result, err := p.StepReviewPR(cmd.Context(), storyKey)
			if err != nil {
				app.Printer.Text(fmt.Sprintf("Error: %s", err))
				return NewExitError(1)
			}

			if !result.Success {
				app.Printer.Text(fmt.Sprintf("Review failed: %s", result.Reason))
				if result.PRURL != "" {
					app.Printer.Text(fmt.Sprintf("PR: %s", result.PRURL))
				}
				return NewExitError(1)
			}

			msg := fmt.Sprintf("Story %s: PR reviewed and merged", storyKey)
			if result.PRURL != "" {
				msg += fmt.Sprintf(" (%s)", result.PRURL)
			}
			app.Printer.Text(msg)
			return nil
		},
	}
}
