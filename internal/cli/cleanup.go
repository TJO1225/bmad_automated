package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"story-factory/internal/git"
)

// newCleanupCommand creates the `cleanup` subcommand.
//
// Iterates every worktree under .story-factory/worktrees/, checks whether
// the associated story branch has been merged into the project's default
// branch, and removes merged worktrees + branches. Unmerged worktrees are
// left alone with a warning so no in-flight work is lost.
func newCleanupCommand(app *App) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Remove worktrees for stories whose PRs have merged",
		Long: `Walk .story-factory/worktrees/ and remove each worktree whose branch has been
merged into the project's default branch. Unmerged worktrees are preserved
unless --force is set.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			projectDir, err := app.ResolveProjectDir()
			if err != nil {
				return fmt.Errorf("failed to determine working directory: %w", err)
			}

			ctx := cmd.Context()
			baseBranch, err := git.DefaultBranch(ctx, projectDir)
			if err != nil {
				return fmt.Errorf("resolve default branch: %w", err)
			}

			worktrees, err := git.ListWorktrees(ctx, projectDir)
			if err != nil {
				return fmt.Errorf("list worktrees: %w", err)
			}

			root := filepath.Join(projectDir, ".story-factory", "worktrees")
			var removed, skipped, failed int

			for _, wt := range worktrees {
				// Only touch worktrees under our managed root.
				if !strings.HasPrefix(wt.Path, root) {
					continue
				}
				// Only touch story branches story-factory created.
				if !strings.HasPrefix(wt.Branch, "story/") {
					continue
				}

				merged := force
				if !force {
					m, err := git.IsBranchMerged(ctx, projectDir, wt.Branch, baseBranch)
					if err != nil {
						app.Printer.Text(fmt.Sprintf("  %s: merged-check failed: %v", wt.Branch, err))
						failed++
						continue
					}
					merged = m
				}

				if !merged {
					app.Printer.Text(fmt.Sprintf("  %s: unmerged — skipping (use --force to override)", wt.Branch))
					skipped++
					continue
				}

				if err := git.RemoveWorktree(ctx, projectDir, wt.Path); err != nil {
					app.Printer.Text(fmt.Sprintf("  %s: remove worktree: %v", wt.Path, err))
					failed++
					continue
				}
				if err := git.DeleteBranch(ctx, projectDir, wt.Branch, force); err != nil {
					// Not fatal — the worktree is gone; branch may have been
					// auto-deleted by `worktree remove` in some git versions.
					app.Printer.Text(fmt.Sprintf("  %s: delete branch: %v", wt.Branch, err))
				}
				app.Printer.Text(fmt.Sprintf("  %s: removed (%s)", wt.Branch, wt.Path))
				removed++
			}

			// Best-effort: prune empty root dir.
			if removed > 0 {
				_ = os.Remove(root)
			}

			app.Printer.Text(fmt.Sprintf("Cleanup: %d removed, %d skipped, %d failed", removed, skipped, failed))
			if failed > 0 {
				return NewExitError(1)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Remove worktrees even if their branch is unmerged (use with care)")
	return cmd
}
