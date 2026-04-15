package pipeline

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"story-factory/internal/status"
)

// setupMinimalSprintStatus creates a minimal sprint-status.yaml in a temp directory
// at the DefaultStatusPath location.
func setupMinimalSprintStatus(t *testing.T, dir string) {
	t.Helper()
	statusDir := filepath.Dir(filepath.Join(dir, status.DefaultStatusPath))
	require.NoError(t, os.MkdirAll(statusDir, 0755))
	require.NoError(t, os.WriteFile(
		filepath.Join(dir, status.DefaultStatusPath),
		[]byte("development_status: {}\n"),
		0644,
	))
}

// setupBMADAgents creates the .claude/skills/ directories required for tests.
// By default it creates bmad-create-story, bmad-dev-story, and bmad-code-review
// so BMAD-mode checks pass. Tests can pass a shorter list for negative cases.
func setupBMADAgents(t *testing.T, dir string, skills ...string) {
	t.Helper()
	if len(skills) == 0 {
		skills = []string{"bmad-create-story", "bmad-dev-story", "bmad-code-review"}
	}
	for _, name := range skills {
		agentDir := filepath.Join(dir, ".claude", "skills", name)
		require.NoError(t, os.MkdirAll(agentDir, 0755))
	}
}

// --- CheckBdCLI tests ---

func TestCheckBdCLI_ReturnsCorrectErrorType(t *testing.T) {
	err := CheckBdCLI()
	if err != nil {
		// bd is not on PATH — verify error type
		assert.True(t, errors.Is(err, ErrPreconditionFailed))
		var precondErr *PreconditionError
		require.True(t, errors.As(err, &precondErr))
		assert.Equal(t, "bd-cli", precondErr.Check)
		assert.Contains(t, precondErr.Detail, "bd CLI not found")
	}
	// If err is nil, bd is on PATH — that's fine too
}

// --- CheckSprintStatus tests ---

func TestCheckSprintStatus(t *testing.T) {
	tests := []struct {
		name            string
		setup           func(t *testing.T, dir string)
		wantErr         bool
		wantCheck       string
		detailSubstring string
	}{
		{
			name: "passes when sprint-status.yaml exists",
			setup: func(t *testing.T, dir string) {
				setupMinimalSprintStatus(t, dir)
			},
			wantErr: false,
		},
		{
			name:            "fails when sprint-status.yaml missing",
			setup:           func(t *testing.T, dir string) {},
			wantErr:         true,
			wantCheck:       "sprint-status",
			detailSubstring: "sprint-status.yaml not found",
		},
		{
			name: "fails when sprint-status.yaml is a directory",
			setup: func(t *testing.T, dir string) {
				p := filepath.Join(dir, status.DefaultStatusPath)
				require.NoError(t, os.MkdirAll(p, 0755))
			},
			wantErr:         true,
			wantCheck:       "sprint-status",
			detailSubstring: "not a regular file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)

			err := CheckSprintStatus(dir)
			if tt.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrPreconditionFailed))
				var precondErr *PreconditionError
				require.True(t, errors.As(err, &precondErr))
				assert.Equal(t, tt.wantCheck, precondErr.Check)
				assert.Contains(t, precondErr.Detail, tt.detailSubstring)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- CheckBMADAgents tests ---

func TestCheckBMADAgents(t *testing.T) {
	tests := []struct {
		name            string
		mode            string
		setup           func(t *testing.T, dir string)
		wantErr         bool
		detailSubstring string
	}{
		{
			name: "bmad mode passes when all three skills exist",
			mode: "bmad",
			setup: func(t *testing.T, dir string) {
				setupBMADAgents(t, dir)
			},
			wantErr: false,
		},
		{
			name: "beads mode passes with only create-story",
			mode: "beads",
			setup: func(t *testing.T, dir string) {
				setupBMADAgents(t, dir, "bmad-create-story")
			},
			wantErr: false,
		},
		{
			name:            "bmad mode fails when create-story missing",
			mode:            "bmad",
			setup:           func(t *testing.T, dir string) {},
			wantErr:         true,
			detailSubstring: "bmad-create-story/ not found",
		},
		{
			name: "bmad mode fails when dev-story missing",
			mode: "bmad",
			setup: func(t *testing.T, dir string) {
				setupBMADAgents(t, dir, "bmad-create-story", "bmad-code-review")
			},
			wantErr:         true,
			detailSubstring: "bmad-dev-story/ not found",
		},
		{
			name: "bmad mode fails when code-review missing",
			mode: "bmad",
			setup: func(t *testing.T, dir string) {
				setupBMADAgents(t, dir, "bmad-create-story", "bmad-dev-story")
			},
			wantErr:         true,
			detailSubstring: "bmad-code-review/ not found",
		},
		{
			name: "fails when bmad-create-story path is a file",
			mode: "bmad",
			setup: func(t *testing.T, dir string) {
				p := filepath.Join(dir, ".claude", "skills", "bmad-create-story")
				require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
				require.NoError(t, os.WriteFile(p, []byte("x"), 0644))
			},
			wantErr:         true,
			detailSubstring: "is not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)

			err := CheckBMADAgents(dir, tt.mode)
			if tt.wantErr {
				require.Error(t, err)
				assert.True(t, errors.Is(err, ErrPreconditionFailed))
				var precondErr *PreconditionError
				require.True(t, errors.As(err, &precondErr))
				assert.Equal(t, "bmad-agents", precondErr.Check)
				assert.Contains(t, precondErr.Detail, tt.detailSubstring)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// --- CheckCleanWorkingTree tests ---

func TestCheckCleanWorkingTree_CleanRepo(t *testing.T) {
	dir := initTestGitRepo(t)
	assert.NoError(t, CheckCleanWorkingTree(dir))
}

func TestCheckCleanWorkingTree_DirtyRepo(t *testing.T) {
	dir := initTestGitRepo(t)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "dirty.txt"), []byte("x"), 0644))

	err := CheckCleanWorkingTree(dir)
	require.Error(t, err)
	var precondErr *PreconditionError
	require.True(t, errors.As(err, &precondErr))
	assert.Equal(t, "clean-tree", precondErr.Check)
	assert.Contains(t, precondErr.Detail, "uncommitted changes")
}

func TestCheckCleanWorkingTree_NotARepo(t *testing.T) {
	dir := t.TempDir()
	err := CheckCleanWorkingTree(dir)
	require.Error(t, err)
	var precondErr *PreconditionError
	require.True(t, errors.As(err, &precondErr))
	assert.Equal(t, "clean-tree", precondErr.Check)
	assert.Contains(t, precondErr.Detail, "not a git repository")
}

// --- CheckAll tests ---

func TestCheckAll_BmadMode_SuccessWhenAllPresent(t *testing.T) {
	dir := initTestGitRepo(t)
	setupMinimalSprintStatus(t, dir)
	setupBMADAgents(t, dir)
	commitAll(t, dir, "fixture setup")

	// BMAD mode requires gh on PATH and a clean git tree. If gh is missing
	// (e.g. in sandboxed CI) we tolerate that — the point of this test is
	// that sprint-status + agents + clean tree all pass and the failure,
	// if any, is the gh CLI check.
	err := CheckAll(dir, "bmad")
	if err != nil {
		var precondErr *PreconditionError
		require.True(t, errors.As(err, &precondErr))
		assert.Equal(t, "gh-cli", precondErr.Check, "only gh-cli may legitimately fail in CI")
	}
}

// initTestGitRepo initializes a real git repo in a temp dir so the
// clean-working-tree precondition has a valid target to inspect.
func initTestGitRepo(t *testing.T) string {
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

// commitAll stages all changes in dir and creates a commit with the given
// message, so tests that set up fixture files after initTestGitRepo still
// see a clean working tree.
func commitAll(t *testing.T, dir, message string) {
	t.Helper()
	for _, args := range [][]string{
		{"add", "-A"},
		{"commit", "-m", message},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		require.NoError(t, err, "git %v failed: %s", args, out)
	}
}

func TestCheckAll_BeadsMode_ChecksBdCLI(t *testing.T) {
	dir := t.TempDir()
	setupMinimalSprintStatus(t, dir)
	setupBMADAgents(t, dir, "bmad-create-story")

	err := CheckAll(dir, "beads")
	if err != nil {
		var precondErr *PreconditionError
		require.True(t, errors.As(err, &precondErr))
		// bd-cli is the only remaining check that might fail in CI.
		assert.Equal(t, "bd-cli", precondErr.Check)
	}
}

func TestCheckAll_FailsOnMissingSprintStatus(t *testing.T) {
	dir := initTestGitRepo(t)
	setupBMADAgents(t, dir)

	err := CheckAll(dir, "bmad")
	require.Error(t, err)

	var precondErr *PreconditionError
	require.True(t, errors.As(err, &precondErr))
	assert.Equal(t, "sprint-status", precondErr.Check)
}

func TestCheckAll_FailsOnMissingBMADAgents(t *testing.T) {
	dir := initTestGitRepo(t)
	setupMinimalSprintStatus(t, dir)

	err := CheckAll(dir, "bmad")
	require.Error(t, err)

	var precondErr *PreconditionError
	require.True(t, errors.As(err, &precondErr))
	assert.Equal(t, "bmad-agents", precondErr.Check)
}

func TestCheckAll_ReturnsFirstFailure(t *testing.T) {
	// Empty dir in bmad mode — sprint-status is checked first.
	dir := initTestGitRepo(t)

	err := CheckAll(dir, "bmad")
	require.Error(t, err)

	var precondErr *PreconditionError
	require.True(t, errors.As(err, &precondErr))
	assert.Equal(t, "sprint-status", precondErr.Check)
}

// --- Error wrapping tests ---

func TestPreconditionError_WrapsErrPreconditionFailed(t *testing.T) {
	err := &PreconditionError{
		Check:  "test-check",
		Detail: "test detail",
	}

	assert.True(t, errors.Is(err, ErrPreconditionFailed))
	assert.Contains(t, err.Error(), "precondition failed")
	assert.Contains(t, err.Error(), "test-check")
	assert.Contains(t, err.Error(), "test detail")
}

func TestPreconditionError_ErrorsAs(t *testing.T) {
	var wrapped error = &PreconditionError{
		Check:  "test-check",
		Detail: "test detail",
	}

	var precondErr *PreconditionError
	require.True(t, errors.As(wrapped, &precondErr))
	assert.Equal(t, "test-check", precondErr.Check)
	assert.Equal(t, "test detail", precondErr.Detail)
}
