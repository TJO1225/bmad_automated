package pipeline

import (
	"errors"
	"os"
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

// setupBMADAgents creates the .claude/skills/bmad-create-story/ directory.
func setupBMADAgents(t *testing.T, dir string) {
	t.Helper()
	agentDir := filepath.Join(dir, ".claude", "skills", "bmad-create-story")
	require.NoError(t, os.MkdirAll(agentDir, 0755))
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
		setup           func(t *testing.T, dir string)
		wantErr         bool
		wantCheck       string
		detailSubstring string
	}{
		{
			name: "passes when bmad-create-story dir exists",
			setup: func(t *testing.T, dir string) {
				setupBMADAgents(t, dir)
			},
			wantErr: false,
		},
		{
			name:            "fails when bmad-create-story dir missing",
			setup:           func(t *testing.T, dir string) {},
			wantErr:         true,
			wantCheck:       "bmad-agents",
			detailSubstring: "BMAD agent files not found",
		},
		{
			name: "fails when bmad-create-story path is a file",
			setup: func(t *testing.T, dir string) {
				p := filepath.Join(dir, ".claude", "skills", "bmad-create-story")
				require.NoError(t, os.MkdirAll(filepath.Dir(p), 0755))
				require.NoError(t, os.WriteFile(p, []byte("x"), 0644))
			},
			wantErr:         true,
			wantCheck:       "bmad-agents",
			detailSubstring: "not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			tt.setup(t, dir)

			err := CheckBMADAgents(dir)
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

// --- CheckAll tests ---

func TestCheckAll_SuccessWhenAllPresent(t *testing.T) {
	dir := t.TempDir()
	setupMinimalSprintStatus(t, dir)
	setupBMADAgents(t, dir)

	// CheckAll also calls CheckBdCLI which depends on real PATH.
	// If bd is not installed, we expect a bd-cli error first.
	err := CheckAll(dir)
	if err != nil {
		var precondErr *PreconditionError
		require.True(t, errors.As(err, &precondErr))
		// Only bd-cli failure is acceptable here — the other two are set up.
		assert.Equal(t, "bd-cli", precondErr.Check)
	}
}

func TestCheckAll_FailsOnMissingSprintStatus(t *testing.T) {
	dir := t.TempDir()
	// Set up agents but NOT sprint-status
	setupBMADAgents(t, dir)

	err := CheckAll(dir)
	require.Error(t, err)

	var precondErr *PreconditionError
	require.True(t, errors.As(err, &precondErr))
	// Should fail on either bd-cli (if not installed) or sprint-status
	assert.Contains(t, []string{"bd-cli", "sprint-status"}, precondErr.Check)
}

func TestCheckAll_FailsOnMissingBMADAgents(t *testing.T) {
	dir := t.TempDir()
	// Set up sprint-status but NOT agents
	setupMinimalSprintStatus(t, dir)

	err := CheckAll(dir)
	require.Error(t, err)

	var precondErr *PreconditionError
	require.True(t, errors.As(err, &precondErr))
	// Should fail on either bd-cli (if not installed) or bmad-agents
	assert.Contains(t, []string{"bd-cli", "bmad-agents"}, precondErr.Check)
}

func TestCheckAll_ReturnsFirstFailure(t *testing.T) {
	// Empty dir — both sprint-status and agents are missing.
	// First non-bd check to fail should be sprint-status (checked before agents).
	dir := t.TempDir()

	err := CheckAll(dir)
	require.Error(t, err)

	var precondErr *PreconditionError
	require.True(t, errors.As(err, &precondErr))
	// Should fail on bd-cli (first check) or sprint-status (second check)
	assert.Contains(t, []string{"bd-cli", "sprint-status"}, precondErr.Check)
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
