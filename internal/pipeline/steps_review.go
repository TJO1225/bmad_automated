package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"story-factory/internal/config"
)

// StepReviewPR runs a code review on an existing PR using a (typically
// different) LLM backend, then auto-merges on a clean review.
//
// This step is designed to be the final step in the bmad pipeline, after
// open-pr has pushed the branch and created the pull request. Using a
// different backend for review than for development ensures diversity
// of perspective.
//
// Preconditions:
//   - A PR must exist for branch "story/<key>" (created by open-pr).
//   - The review backend's binary must be on PATH.
//
// Actions:
//  1. Look up the PR URL via `gh pr view`.
//  2. Resolve the executor for this step (may be different from dev backend).
//  3. Build a review prompt (backend-specific via BackendPrompts).
//  4. Execute the review with timeout.
//  5. On exit code 0: auto-merge via `gh pr merge --squash` (no --delete-branch:
//     deleting the local head branch makes gh/git touch the default branch, which
//     fails when projectDir is a secondary worktree and main is checked out elsewhere).
//
// Resume logic:
//   - PR already merged → skip, return success.
//   - No PR for branch → fail with actionable message.
//
// Non-retryable: reviewing the same PR twice produces the same result.
func (p *Pipeline) StepReviewPR(ctx context.Context, key string) (StepResult, error) {
	start := time.Now()

	if p.dryRun {
		msg := "dry-run: would review and merge PR for " + storyBranchPrefix + key
		if p.printer != nil {
			p.printer.Text(msg)
		}
		return StepResult{
			Name:    stepNameReviewPR,
			Success: true,
			Reason:  msg,
		}, nil
	}

	branchName := storyBranchPrefix + key

	// Resume: if the PR is already merged, skip.
	if merged, err := isPRMerged(ctx, p.projectDir, branchName); err == nil && merged {
		if p.printer != nil {
			p.printer.Text("review-pr: PR already merged for " + branchName)
		}
		return StepResult{
			Name:     stepNameReviewPR,
			Success:  true,
			Reason:   "PR already merged",
			Duration: time.Since(start),
		}, nil
	}

	// Look up the PR URL.
	prURL, exists := existingPRForBranch(ctx, p.projectDir, branchName)
	if !exists {
		return StepResult{
			Name:     stepNameReviewPR,
			Success:  false,
			Reason:   fmt.Sprintf("review-pr %s: no PR found for branch %s — run open-pr first", key, branchName),
			Duration: time.Since(start),
		}, nil
	}

	if p.printer != nil {
		p.printer.Text("Reviewing PR: " + prURL)
	}

	// Resolve executor and backend name for this step.
	exec := p.resolveExecutor(stepNameReviewPR)
	backendName := p.resolveBackendName(stepNameReviewPR)

	// Build the review prompt with PR URL.
	prompt, err := p.cfg.GetPromptWithData("review-pr", backendName, config.PromptData{
		StoryKey: key,
		PRURL:    prURL,
	})
	if err != nil {
		return StepResult{}, fmt.Errorf("review-pr %s: %w", key, err)
	}

	// Execute the review.
	timeoutCtx, cancel := context.WithTimeout(ctx, p.llmTimeout())
	defer cancel()

	handler := p.verboseHandler()
	exitCode, err := exec.ExecuteWithResult(timeoutCtx, prompt, handler)
	if err != nil {
		if reason := classifyContextErr(err, timeoutCtx, "review-pr "+key); reason != "" {
			return StepResult{
				Name:     stepNameReviewPR,
				Success:  false,
				Reason:   reason,
				Duration: time.Since(start),
			}, nil
		}
		return StepResult{}, err
	}
	if exitCode != 0 {
		return StepResult{
			Name:     stepNameReviewPR,
			Success:  false,
			Reason:   fmt.Sprintf("review-pr %s: reviewer exited with code %d — review may have found issues", key, exitCode),
			PRURL:    prURL,
			Duration: time.Since(start),
		}, nil
	}

	// Review passed — auto-merge.
	if p.printer != nil {
		p.printer.Text("Review passed, merging PR: " + prURL)
	}

	if err := p.mergePR(ctx, prURL); err != nil {
		return StepResult{
			Name:     stepNameReviewPR,
			Success:  false,
			Reason:   fmt.Sprintf("review-pr %s: merge failed: %v", key, err),
			PRURL:    prURL,
			Duration: time.Since(start),
		}, nil
	}

	if p.printer != nil {
		p.printer.Text("PR merged: " + prURL)
	}

	return StepResult{
		Name:     stepNameReviewPR,
		Success:  true,
		PRURL:    prURL,
		Duration: time.Since(start),
	}, nil
}

// mergePR squash-merges a pull request on GitHub. We intentionally omit
// gh's --delete-branch: it updates local git state and can fail with
// "main is already used by worktree" when story-factory runs from a linked
// worktree while the primary clone has main checked out.
func (p *Pipeline) mergePR(ctx context.Context, prURL string) error {
	cmd := exec.CommandContext(ctx, "gh", "pr", "merge", prURL,
		"--squash",
	)
	cmd.Dir = p.projectDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return fmt.Errorf("%w: %s", err, msg)
		}
		return err
	}
	return nil
}

// isPRMerged returns true if the PR for the given branch has already been
// merged. Returns (false, nil) if the PR exists but is open, and (false, err)
// if no PR is found or gh fails.
func isPRMerged(ctx context.Context, projectDir, branch string) (bool, error) {
	cmd := exec.CommandContext(ctx, "gh", "pr", "view", branch,
		"--json", "state", "-q", ".state",
	)
	cmd.Dir = projectDir
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return false, err
	}
	state := strings.TrimSpace(stdout.String())
	return state == "MERGED", nil
}
