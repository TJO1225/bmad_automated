package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// initTestRepoWithCommit creates a test repo with one real commit so
// worktrees can branch off something meaningful.
func initTestRepoWithCommit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-b", "main"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README"), []byte("hello"), 0644))
	for _, args := range [][]string{
		{"add", "README"},
		{"commit", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v: %s", args, out)
	}
	return dir
}

func TestAddAndRemoveWorktree(t *testing.T) {
	repo := initTestRepoWithCommit(t)
	wtPath := filepath.Join(t.TempDir(), "sf-1")
	ctx := context.Background()

	require.NoError(t, AddWorktree(ctx, repo, wtPath, "story/1-1-foo", "main"))

	// Verify the worktree exists and is on the expected branch.
	branch, err := CurrentBranch(ctx, wtPath)
	require.NoError(t, err)
	assert.Equal(t, "story/1-1-foo", branch)

	// ListWorktrees should report two entries now (primary + new).
	list, err := ListWorktrees(ctx, repo)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(list), 2)

	var found bool
	for _, wt := range list {
		if wt.Path == wtPath {
			found = true
			assert.Equal(t, "story/1-1-foo", wt.Branch)
		}
	}
	assert.True(t, found, "new worktree should be in list")

	// Remove it and confirm the directory is gone.
	require.NoError(t, RemoveWorktree(ctx, repo, wtPath))
	_, statErr := os.Stat(wtPath)
	assert.True(t, os.IsNotExist(statErr), "worktree dir should be removed")
}

func TestIsBranchMerged(t *testing.T) {
	repo := initTestRepoWithCommit(t)
	ctx := context.Background()

	// Create a branch with one commit that diverges from main.
	cmd := exec.Command("git", "-C", repo, "checkout", "-b", "feature")
	require.NoError(t, cmd.Run())
	require.NoError(t, os.WriteFile(filepath.Join(repo, "f.txt"), []byte("x"), 0644))
	for _, args := range [][]string{
		{"add", "f.txt"},
		{"commit", "-m", "feature commit"},
	} {
		cmd := exec.Command("git", "-C", repo)
		cmd.Args = append(cmd.Args, args...)
		require.NoError(t, cmd.Run())
	}

	// feature is NOT merged into main yet.
	merged, err := IsBranchMerged(ctx, repo, "feature", "main")
	require.NoError(t, err)
	assert.False(t, merged)

	// Merge it and re-check.
	require.NoError(t, exec.Command("git", "-C", repo, "checkout", "main").Run())
	require.NoError(t, exec.Command("git", "-C", repo, "merge", "--no-ff", "-m", "merge feature", "feature").Run())

	merged, err = IsBranchMerged(ctx, repo, "feature", "main")
	require.NoError(t, err)
	assert.True(t, merged)
}

func TestDeleteBranch(t *testing.T) {
	repo := initTestRepoWithCommit(t)
	ctx := context.Background()

	require.NoError(t, exec.Command("git", "-C", repo, "branch", "temp").Run())
	require.NoError(t, DeleteBranch(ctx, repo, "temp", false))

	exists, err := BranchExists(ctx, repo, "temp")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestCurrentBranchName_DetachedHead(t *testing.T) {
	repo := initTestRepoWithCommit(t)
	ctx := context.Background()

	// Detach HEAD by checking out the commit directly.
	sha, err := exec.Command("git", "-C", repo, "rev-parse", "HEAD").Output()
	require.NoError(t, err)
	require.NoError(t, exec.Command("git", "-C", repo, "checkout", string(sha[:len(sha)-1])).Run())

	name, err := CurrentBranchName(ctx, repo)
	require.NoError(t, err)
	assert.Equal(t, "", name, "detached HEAD should return empty without error")
}

func TestCurrentBranchName_OnBranch(t *testing.T) {
	repo := initTestRepoWithCommit(t)
	ctx := context.Background()

	name, err := CurrentBranchName(ctx, repo)
	require.NoError(t, err)
	assert.Equal(t, "main", name)
}
