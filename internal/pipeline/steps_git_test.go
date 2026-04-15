package pipeline

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"story-factory/internal/config"
	"story-factory/internal/status"
)

// newGitTestRepo creates a temp git repo and returns its path. Tests that
// exercise commit-branch / open-pr need this so the git helpers operate on
// real state.
func newGitTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
		{"commit", "--allow-empty", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, out)
	}
	return dir
}

// writeStoryFile creates a BMAD-shaped story file at the expected path and
// populates sprint-status.yaml with the given key/status so
// ResolveStoryLocation works.
func writeStoryFile(t *testing.T, dir, key, storyStatus, title string) {
	t.Helper()
	storyDir := filepath.Join(dir, "_bmad-output", "implementation-artifacts")
	require.NoError(t, os.MkdirAll(storyDir, 0755))
	content := fmt.Sprintf(`# Story 1.2: %s

Status: %s

## Acceptance Criteria

1. Given a feature request
   When the user runs the command
   Then the output is correct
`, title, storyStatus)
	require.NoError(t, os.WriteFile(filepath.Join(storyDir, key+".md"), []byte(content), 0644))

	statusPath := filepath.Join(dir, status.DefaultStatusPath)
	require.NoError(t, os.WriteFile(statusPath, []byte(
		`story_location: "{project-root}/_bmad-output/implementation-artifacts"
development_status:
  `+key+`: `+storyStatus+"\n"), 0644))
}

// commitFixtures stages everything in dir and commits it so tests can
// exercise commit-branch from a genuinely clean working tree.
func commitFixturesGit(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "-m", "fixture setup"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, out)
	}
}

func TestStepCommitBranch_Success(t *testing.T) {
	dir := newGitTestRepo(t)
	key := "1-2-database-schema"
	writeStoryFile(t, dir, key, "review", "Database Schema")
	commitFixturesGit(t, dir)

	// Simulate dev-story having modified a file.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "feature.go"), []byte("package x\n"), 0644))

	p := NewPipeline(nil, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
	)

	result, err := p.StepCommitBranch(context.Background(), key)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Equal(t, stepNameCommitBranch, result.Name)

	// Assertions on the resulting git state:
	branch := runGitForTest(t, dir, "rev-parse", "--abbrev-ref", "HEAD")
	assert.Equal(t, "story/"+key, branch)

	log := runGitForTest(t, dir, "log", "-1", "--format=%s")
	assert.Contains(t, log, "feat("+key+"): Database Schema")
}

func TestStepCommitBranch_WrongBranch(t *testing.T) {
	dir := newGitTestRepo(t)
	key := "1-2-database-schema"
	writeStoryFile(t, dir, key, "review", "Database Schema")
	commitFixturesGit(t, dir)

	// Switch to a non-default branch to simulate a misconfigured worktree.
	require.NoError(t, exec.Command("git", "-C", dir, "checkout", "-b", "other").Run())

	p := NewPipeline(nil, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
	)

	result, err := p.StepCommitBranch(context.Background(), key)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Reason, "must start from the default branch")
}

func TestStepCommitBranch_BranchAlreadyExists(t *testing.T) {
	dir := newGitTestRepo(t)
	key := "1-2-database-schema"
	writeStoryFile(t, dir, key, "review", "Database Schema")
	commitFixturesGit(t, dir)

	// Pre-create the story branch (simulating a prior interrupted run).
	require.NoError(t, exec.Command("git", "-C", dir, "branch", "story/"+key).Run())

	p := NewPipeline(nil, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
	)

	result, err := p.StepCommitBranch(context.Background(), key)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Reason, "already exists")
}

func TestStepCommitBranch_DryRun(t *testing.T) {
	dir := newGitTestRepo(t)
	key := "1-2-database-schema"

	p := NewPipeline(nil, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
		WithDryRun(true),
	)

	result, err := p.StepCommitBranch(context.Background(), key)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Reason, "dry-run")

	// No branch should have been created.
	out, err := exec.Command("git", "-C", dir, "branch", "--list", "story/"+key).Output()
	require.NoError(t, err)
	assert.Empty(t, out)
}

func TestStepOpenPR_DryRun(t *testing.T) {
	dir := newGitTestRepo(t)
	key := "1-2-database-schema"

	p := NewPipeline(nil, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
		WithDryRun(true),
	)

	result, err := p.StepOpenPR(context.Background(), key)
	require.NoError(t, err)
	assert.True(t, result.Success)
	assert.Contains(t, result.Reason, "dry-run")
	assert.Empty(t, result.PRURL)
}

func TestStepOpenPR_WrongBranch(t *testing.T) {
	dir := newGitTestRepo(t)
	key := "1-2-database-schema"
	writeStoryFile(t, dir, key, "review", "Database Schema")
	commitFixturesGit(t, dir)

	// Still on main (not on story/<key>) — open-pr should refuse.
	p := NewPipeline(nil, config.DefaultConfig(), dir,
		WithStatus(status.NewReader(dir)),
	)

	result, err := p.StepOpenPR(context.Background(), key)
	require.NoError(t, err)
	assert.False(t, result.Success)
	assert.Contains(t, result.Reason, "expected branch")
	assert.Contains(t, result.Reason, "story/"+key)
}

// runGitForTest runs git with the given args in dir and returns trimmed
// stdout. Fails the test on any error. Used by tests to inspect repo state.
func runGitForTest(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %v failed: %s", args, out)
	return string(trimNewline(out))
}

// trimNewline strips the single trailing newline git appends to output.
func trimNewline(b []byte) []byte {
	if len(b) > 0 && b[len(b)-1] == '\n' {
		return b[:len(b)-1]
	}
	return b
}
