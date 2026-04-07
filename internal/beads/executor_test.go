package beads

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBeadID(t *testing.T) {
	tests := []struct {
		name   string
		stdout string
		want   string
	}{
		{
			name:   "single line",
			stdout: "bd-abc123\n",
			want:   "bd-abc123",
		},
		{
			name:   "leading blank lines",
			stdout: "\n\nbd-xyz789\n",
			want:   "bd-xyz789",
		},
		{
			name:   "trailing whitespace",
			stdout: "  bd-trimmed  \n",
			want:   "bd-trimmed",
		},
		{
			name:   "multiple lines takes first",
			stdout: "bd-first\nsome other output\n",
			want:   "bd-first",
		},
		{
			name:   "empty stdout",
			stdout: "",
			want:   "",
		},
		{
			name:   "whitespace only",
			stdout: "   \n  \n",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseBeadID(tt.stdout)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestAppendTrackingComment(t *testing.T) {
	t.Run("appends comment to file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "story.md")
		require.NoError(t, os.WriteFile(path, []byte("# Story 1.2: Test\n"), 0644))

		err := AppendTrackingComment(path, "bd-abc123")
		require.NoError(t, err)

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.Contains(t, string(content), "<!-- bead:bd-abc123 -->")
	})

	t.Run("does not duplicate existing comment", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "story.md")
		initial := "# Story 1.2: Test\n\n<!-- bead:bd-abc123 -->\n"
		require.NoError(t, os.WriteFile(path, []byte(initial), 0644))

		err := AppendTrackingComment(path, "bd-abc123")
		require.NoError(t, err)

		content, err := os.ReadFile(path)
		require.NoError(t, err)
		// Should still have exactly one occurrence
		assert.Equal(t, initial, string(content))
	})

	t.Run("returns error for non-existent file", func(t *testing.T) {
		err := AppendTrackingComment("/no/such/file.md", "bd-abc123")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "read story file")
	})
}

func TestMockExecutor_RecordsCalls(t *testing.T) {
	mock := &MockExecutor{BeadID: "bd-test-123"}

	id, err := mock.Create(context.Background(), "1-2-schema", "Database Schema", "ACs here", nil)
	require.NoError(t, err)
	assert.Equal(t, "bd-test-123", id)

	require.Len(t, mock.Calls, 1)
	assert.Equal(t, "1-2-schema", mock.Calls[0].Key)
	assert.Equal(t, "Database Schema", mock.Calls[0].Title)
	assert.Equal(t, "ACs here", mock.Calls[0].ACs)
}

func TestMockExecutor_ReturnsError(t *testing.T) {
	mock := &MockExecutor{Err: assert.AnError}

	id, err := mock.Create(context.Background(), "key", "title", "acs", nil)
	require.Error(t, err)
	assert.Empty(t, id)
	require.Len(t, mock.Calls, 1)
}
