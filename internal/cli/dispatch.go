package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"story-factory/internal/git"
	"story-factory/internal/status"
	"story-factory/internal/tmux"
)

// newDispatchCommand creates the `dispatch` subcommand.
//
// Usage:
//
//	story-factory dispatch [story-keys...] --parallel N [--mode bmad]
//
// With no keys, dispatch pulls all backlog stories from sprint-status.yaml,
// the same way `queue` does. Dispatch refuses to run outside tmux because
// each parallel story is delivered to its own tmux pane.
func newDispatchCommand(app *App) *cobra.Command {
	var parallel int

	cmd := &cobra.Command{
		Use:   "dispatch [story-keys...]",
		Short: "Run stories in parallel across tmux panes (git worktree per story)",
		Long: `Dispatch runs the pipeline for each requested story in its own git worktree,
with each worktree's execution displayed in a dedicated tmux pane (2x2 per
window, configurable via --parallel).

Must be run from inside a tmux session. Each story gets a fresh worktree at
.story-factory/worktrees/<key> based on the project's default branch.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := app.RunPreconditions(); err != nil {
				return err
			}

			projectDir, err := app.ResolveProjectDir()
			if err != nil {
				return fmt.Errorf("failed to determine working directory: %w", err)
			}

			// Resolve story keys: explicit args, or all backlog stories.
			reader := status.NewReader(projectDir)
			storyKeys, err := resolveDispatchKeys(reader, args)
			if err != nil {
				app.Printer.Text(fmt.Sprintf("Error: %s", err))
				return NewExitError(1)
			}
			if len(storyKeys) == 0 {
				app.Printer.Text("No backlog stories to dispatch")
				return nil
			}

			// Find the story-factory binary we were invoked as so dispatched
			// panes run the same build.
			binaryPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve story-factory binary path: %w", err)
			}

			// Determine the base branch once so we don't shell out N times.
			baseBranch, err := git.DefaultBranch(cmd.Context(), projectDir)
			if err != nil {
				app.Printer.Text(fmt.Sprintf("Error: %s", err))
				return NewExitError(1)
			}

			worktreeRoot := filepath.Join(projectDir, ".story-factory", "worktrees")

			dispatcher, err := tmux.NewDispatcher(tmux.DispatcherOptions{
				Parallel:     parallel,
				ProjectRoot:  projectDir,
				WorktreeRoot: worktreeRoot,
				BaseBranch:   baseBranch,
				BinaryPath:   binaryPath,
				Mode:         app.Mode,
				Verbose:      app.Verbose,
			}, func(line string) {
				app.Printer.Text(line)
			})
			if err != nil {
				app.Printer.Text(fmt.Sprintf("Error: %s", err))
				return NewExitError(1)
			}

			app.Printer.Text(fmt.Sprintf(
				"Dispatching %d stories across %d parallel slots (worktrees at %s)",
				len(storyKeys), parallel, worktreeRoot,
			))

			summary, err := dispatcher.Run(cmd.Context(), storyKeys)
			if err != nil {
				app.Printer.Text(fmt.Sprintf("Error: %s", err))
				return NewExitError(1)
			}

			printDispatchSummary(app, summary)

			if summary.Failed > 0 {
				return NewExitError(1)
			}
			return nil
		},
	}

	cmd.Flags().IntVarP(&parallel, "parallel", "p", 4, "Max concurrent stories (panes per window = 4)")
	return cmd
}

// resolveDispatchKeys returns the list of story keys to dispatch. If args is
// non-empty, those keys are used verbatim; otherwise the backlog queue is
// pulled from sprint-status.yaml.
func resolveDispatchKeys(reader *status.Reader, args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}
	entries, err := reader.BacklogStories()
	if err != nil {
		return nil, err
	}
	keys := make([]string, len(entries))
	for i, e := range entries {
		keys[i] = e.Key
	}
	return keys, nil
}

// printDispatchSummary renders the dispatcher's BatchSummary using the app's printer.
func printDispatchSummary(app *App, summary tmux.BatchSummary) {
	app.Printer.Divider()
	app.Printer.Text(fmt.Sprintf("Dispatch summary: %d succeeded, %d failed in %s",
		summary.Succeeded, summary.Failed, summary.Duration.Round(1e9)))
	for _, r := range summary.Results {
		switch {
		case r.Err != nil:
			app.Printer.Text(fmt.Sprintf("  %s FAILED: %v", r.Key, r.Err))
		case r.ExitCode != 0:
			app.Printer.Text(fmt.Sprintf("  %s exit=%d (worktree: %s)", r.Key, r.ExitCode, r.WorktreePath))
		default:
			app.Printer.Text(fmt.Sprintf("  %s done (worktree: %s)", r.Key, r.WorktreePath))
		}
	}
	app.Printer.Text(fmt.Sprintf("Worktrees left in place at %s. Run `story-factory cleanup` after PRs merge.", summary.WorktreeDir))
}
