package pipeline

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"story-factory/internal/git"
	"story-factory/internal/storyfile"
)

// storyBranchPrefix is prepended to the story key to produce the git branch
// name (e.g., key "1-2-foo" becomes branch "story/1-2-foo").
const storyBranchPrefix = "story/"

// StepCommitBranch creates a story branch in the project working tree and
// commits all changes made by the dev-story step.
//
// Preconditions:
//   - The working tree must be on the repository's default branch (main
//     or master) so commits don't pile up on an unrelated branch.
//   - The story file must exist so the commit title can be derived from its
//     H1 heading.
//
// Actions:
//   - Creates branch "story/<key>".
//   - Stages all changes (`git add -A`).
//   - Commits with a "feat(<key>): <title>" message plus an AC-derived body.
//
// Non-retryable: a retry would fail on the already-checked-out branch and
// produce confusing output; the non-retryable list in pipeline.go enforces this.
func (p *Pipeline) StepCommitBranch(ctx context.Context, key string) (StepResult, error) {
	start := time.Now()

	if p.dryRun {
		msg := "dry-run: would create branch " + storyBranchPrefix + key + " and commit"
		if p.printer != nil {
			p.printer.Text(msg)
		}
		return StepResult{
			Name:    stepNameCommitBranch,
			Success: true,
			Reason:  msg,
		}, nil
	}

	if p.status == nil {
		return StepResult{}, fmt.Errorf("commit-branch %s: pipeline has no status reader", key)
	}

	// Verify we're on the default branch before branching off.
	defaultBranch, err := git.DefaultBranch(ctx, p.projectDir)
	if err != nil {
		return StepResult{}, fmt.Errorf("commit-branch %s: resolve default branch: %w", key, err)
	}
	current, err := git.CurrentBranch(ctx, p.projectDir)
	if err != nil {
		return StepResult{}, fmt.Errorf("commit-branch %s: %w", key, err)
	}
	if current != defaultBranch {
		return StepResult{
			Name:     stepNameCommitBranch,
			Success:  false,
			Reason:   fmt.Sprintf("commit-branch %s: expected branch %q, found %q — commit-branch must start from the default branch", key, defaultBranch, current),
			Duration: time.Since(start),
		}, nil
	}

	// Load story title + ACs for the commit message.
	title, acs, err := readStoryMeta(p, key)
	if err != nil {
		return StepResult{
			Name:     stepNameCommitBranch,
			Success:  false,
			Reason:   fmt.Sprintf("commit-branch %s: %v", key, err),
			Duration: time.Since(start),
		}, nil
	}

	branchName := storyBranchPrefix + key
	exists, err := git.BranchExists(ctx, p.projectDir, branchName)
	if err != nil {
		return StepResult{}, fmt.Errorf("commit-branch %s: check branch exists: %w", key, err)
	}
	if exists {
		return StepResult{
			Name:     stepNameCommitBranch,
			Success:  false,
			Reason:   fmt.Sprintf("commit-branch %s: branch %q already exists — delete it or resume manually", key, branchName),
			Duration: time.Since(start),
		}, nil
	}

	if err := git.CreateBranch(ctx, p.projectDir, branchName); err != nil {
		return StepResult{}, fmt.Errorf("commit-branch %s: create branch: %w", key, err)
	}

	if err := git.AddAll(ctx, p.projectDir); err != nil {
		return StepResult{}, fmt.Errorf("commit-branch %s: git add: %w", key, err)
	}

	commitMsg := buildCommitMessage(key, title, acs)
	if err := git.Commit(ctx, p.projectDir, commitMsg); err != nil {
		return StepResult{}, fmt.Errorf("commit-branch %s: git commit: %w", key, err)
	}

	if p.printer != nil {
		p.printer.Text(fmt.Sprintf("Committed %s on %s", key, branchName))
	}

	return StepResult{
		Name:     stepNameCommitBranch,
		Success:  true,
		Duration: time.Since(start),
	}, nil
}

// prUrlRegex extracts the final line of gh pr create output that looks like
// a GitHub PR URL. gh emits the URL as the last line on success.
var prUrlRegex = regexp.MustCompile(`https?://\S+/pull/\d+`)

// StepOpenPR pushes the current story branch to origin and opens a pull
// request via `gh pr create`.
//
// Preconditions:
//   - Current branch must be "story/<key>" (produced by StepCommitBranch).
//   - The repository must have an origin remote (checked in preconditions).
//   - The `gh` CLI must be authenticated (caller's responsibility).
//
// The returned StepResult has PRURL populated on success so run/queue/epic
// commands can echo the URL back.
//
// Non-retryable: a second push + gh pr create would fail noisily on the
// duplicate PR.
func (p *Pipeline) StepOpenPR(ctx context.Context, key string) (StepResult, error) {
	start := time.Now()

	if p.dryRun {
		msg := "dry-run: would push " + storyBranchPrefix + key + " and open PR"
		if p.printer != nil {
			p.printer.Text(msg)
		}
		return StepResult{
			Name:    stepNameOpenPR,
			Success: true,
			Reason:  msg,
		}, nil
	}

	branchName := storyBranchPrefix + key
	current, err := git.CurrentBranch(ctx, p.projectDir)
	if err != nil {
		return StepResult{}, fmt.Errorf("open-pr %s: %w", key, err)
	}
	if current != branchName {
		return StepResult{
			Name:     stepNameOpenPR,
			Success:  false,
			Reason:   fmt.Sprintf("open-pr %s: expected branch %q, found %q", key, branchName, current),
			Duration: time.Since(start),
		}, nil
	}

	if err := git.PushUpstream(ctx, p.projectDir, branchName); err != nil {
		return StepResult{
			Name:     stepNameOpenPR,
			Success:  false,
			Reason:   fmt.Sprintf("open-pr %s: git push: %v", key, err),
			Duration: time.Since(start),
		}, nil
	}

	title, acs, err := readStoryMeta(p, key)
	if err != nil {
		return StepResult{
			Name:     stepNameOpenPR,
			Success:  false,
			Reason:   fmt.Sprintf("open-pr %s: %v", key, err),
			Duration: time.Since(start),
		}, nil
	}

	defaultBranch, err := git.DefaultBranch(ctx, p.projectDir)
	if err != nil {
		return StepResult{}, fmt.Errorf("open-pr %s: resolve default branch: %w", key, err)
	}

	prTitle := fmt.Sprintf("feat(%s): %s", key, title)
	prBody := buildPRBody(key, acs)

	cmd := exec.CommandContext(ctx, "gh", "pr", "create",
		"--title", prTitle,
		"--body", prBody,
		"--base", defaultBranch,
		"--head", branchName,
	)
	cmd.Dir = p.projectDir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return StepResult{
			Name:     stepNameOpenPR,
			Success:  false,
			Reason:   fmt.Sprintf("open-pr %s: gh pr create: %v: %s", key, err, strings.TrimSpace(stderr.String())),
			Duration: time.Since(start),
		}, nil
	}

	prURL := prUrlRegex.FindString(stdout.String())
	if prURL == "" {
		// gh should always print the URL on success; if it didn't, treat as
		// operational failure so the user investigates.
		return StepResult{
			Name:     stepNameOpenPR,
			Success:  false,
			Reason:   fmt.Sprintf("open-pr %s: could not parse PR URL from gh output: %s", key, strings.TrimSpace(stdout.String())),
			Duration: time.Since(start),
		}, nil
	}

	if p.printer != nil {
		p.printer.Text("Opened PR: " + prURL)
	}

	return StepResult{
		Name:     stepNameOpenPR,
		Success:  true,
		PRURL:    prURL,
		Duration: time.Since(start),
	}, nil
}

// readStoryMeta loads the story file for key and extracts its title + AC block.
// Missing file or malformed sections surface as operational failures.
func readStoryMeta(p *Pipeline, key string) (title, acs string, err error) {
	storyDir, err := p.status.ResolveStoryLocation(p.projectDir)
	if err != nil {
		return "", "", fmt.Errorf("resolve story location: %w", err)
	}
	storyPath := filepath.Join(storyDir, key+".md")

	content, err := os.ReadFile(storyPath)
	if err != nil {
		return "", "", fmt.Errorf("read story file %s: %w", storyPath, err)
	}
	title, err = storyfile.ExtractTitle(string(content))
	if err != nil {
		return "", "", fmt.Errorf("extract title: %w", err)
	}
	acs, err = storyfile.ExtractAcceptanceCriteria(string(content))
	if err != nil {
		// ACs are best-effort for the commit body; fall back to empty rather
		// than failing the commit step. The title is the required bit.
		return title, "", nil
	}
	return title, acs, nil
}

// buildCommitMessage formats a conventional-commit-style message with the
// story key, title, and a truncated AC summary body.
func buildCommitMessage(key, title, acs string) string {
	subject := fmt.Sprintf("feat(%s): %s", key, title)
	if acs == "" {
		return subject
	}
	body := truncateBlock(acs, 20)
	return subject + "\n\n" + body + "\n\nStory: " + key
}

// buildPRBody renders the pull-request body: the AC block (untruncated) plus
// a link-style pointer to the story file.
func buildPRBody(key, acs string) string {
	var sb strings.Builder
	sb.WriteString("Automated PR from story-factory.\n\n")
	sb.WriteString("Story: `" + key + "`\n\n")
	if acs != "" {
		sb.WriteString("## Acceptance Criteria\n\n")
		sb.WriteString(acs)
		sb.WriteString("\n")
	}
	return sb.String()
}

// truncateBlock returns at most maxLines of block; if truncation happened,
// a "...(N more lines)" marker is appended. Used to keep commit messages
// readable when ACs are long.
func truncateBlock(block string, maxLines int) string {
	lines := strings.Split(block, "\n")
	if len(lines) <= maxLines {
		return block
	}
	head := strings.Join(lines[:maxLines], "\n")
	return head + fmt.Sprintf("\n... (%d more lines)", len(lines)-maxLines)
}
