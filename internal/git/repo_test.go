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

// initTestRepo creates a temp git repo with an initial commit on main. It
// configures user.email/user.name so commits don't fail on fresh CI hosts.
func initTestRepo(t *testing.T) string {
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

func TestCurrentBranch(t *testing.T) {
	dir := initTestRepo(t)
	branch, err := CurrentBranch(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, "main", branch)
}

func TestIsClean(t *testing.T) {
	dir := initTestRepo(t)
	clean, err := IsClean(context.Background(), dir)
	require.NoError(t, err)
	assert.True(t, clean, "fresh repo should be clean")

	require.NoError(t, os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0644))
	clean, err = IsClean(context.Background(), dir)
	require.NoError(t, err)
	assert.False(t, clean, "untracked file should make tree dirty")
}

func TestCreateBranchAndBranchExists(t *testing.T) {
	dir := initTestRepo(t)
	ctx := context.Background()

	exists, err := BranchExists(ctx, dir, "story/1-1-foo")
	require.NoError(t, err)
	assert.False(t, exists)

	require.NoError(t, CreateBranch(ctx, dir, "story/1-1-foo"))

	exists, err = BranchExists(ctx, dir, "story/1-1-foo")
	require.NoError(t, err)
	assert.True(t, exists)

	branch, err := CurrentBranch(ctx, dir)
	require.NoError(t, err)
	assert.Equal(t, "story/1-1-foo", branch)
}

func TestAddAllAndCommit(t *testing.T) {
	dir := initTestRepo(t)
	ctx := context.Background()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("content"), 0644))
	require.NoError(t, AddAll(ctx, dir))
	require.NoError(t, Commit(ctx, dir, "feat(1-1): add file"))

	clean, err := IsClean(ctx, dir)
	require.NoError(t, err)
	assert.True(t, clean, "after commit, tree should be clean again")
}

func TestDefaultBranch_MainFallback(t *testing.T) {
	dir := initTestRepo(t)
	branch, err := DefaultBranch(context.Background(), dir)
	require.NoError(t, err)
	assert.Equal(t, "main", branch)
}

func TestHasRemote_NoRemotes(t *testing.T) {
	dir := initTestRepo(t)
	has, err := HasRemote(context.Background(), dir, "origin")
	require.NoError(t, err)
	assert.False(t, has)
}
