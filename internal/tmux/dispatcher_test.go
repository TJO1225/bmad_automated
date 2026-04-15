package tmux

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseSentinel_Match(t *testing.T) {
	nonce := "abc123"
	captured := "running dev-story...\n__SF_DONE__:abc123:0\n$ "
	exit, ok := ParseSentinel(captured, nonce)
	assert.True(t, ok)
	assert.Equal(t, 0, exit)
}

func TestParseSentinel_NonZeroExit(t *testing.T) {
	captured := "something failed\n__SF_DONE__:deadbeef:2\n"
	exit, ok := ParseSentinel(captured, "deadbeef")
	assert.True(t, ok)
	assert.Equal(t, 2, exit)
}

func TestParseSentinel_WrongNonce(t *testing.T) {
	// Previous dispatch left its sentinel in scrollback — should NOT match.
	captured := "__SF_DONE__:old_nonce:0\n(new command running)\n"
	exit, ok := ParseSentinel(captured, "new_nonce")
	assert.False(t, ok)
	assert.Equal(t, 0, exit)
}

func TestParseSentinel_NoMatch(t *testing.T) {
	_, ok := ParseSentinel("still running...", "anything")
	assert.False(t, ok)
}

func TestShellQuote(t *testing.T) {
	assert.Equal(t, "'simple'", shellQuote("simple"))
	assert.Equal(t, "'/path/with space'", shellQuote("/path/with space"))
	// Single quote inside becomes '\'' via the standard trick.
	assert.Equal(t, `'it'\''s'`, shellQuote("it's"))
}

func TestNewNonce_UniqueAndHex(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 50; i++ {
		n := newNonce()
		assert.Len(t, n, 16, "nonce should be 16 hex chars")
		assert.False(t, seen[n], "nonce should be unique")
		seen[n] = true
	}
}

func TestNewDispatcher_Validation(t *testing.T) {
	cases := []struct {
		name string
		opts DispatcherOptions
		ok   bool
	}{
		{"rejects parallel 0", DispatcherOptions{Parallel: 0, ProjectRoot: "/p", WorktreeRoot: "/w", BaseBranch: "main", BinaryPath: "/bin/sf"}, false},
		{"rejects missing ProjectRoot", DispatcherOptions{Parallel: 1, WorktreeRoot: "/w", BaseBranch: "main", BinaryPath: "/bin/sf"}, false},
		{"rejects missing WorktreeRoot", DispatcherOptions{Parallel: 1, ProjectRoot: "/p", BaseBranch: "main", BinaryPath: "/bin/sf"}, false},
		{"rejects missing BaseBranch", DispatcherOptions{Parallel: 1, ProjectRoot: "/p", WorktreeRoot: "/w", BinaryPath: "/bin/sf"}, false},
		{"rejects missing BinaryPath", DispatcherOptions{Parallel: 1, ProjectRoot: "/p", WorktreeRoot: "/w", BaseBranch: "main"}, false},
		{"accepts full options, defaults mode", DispatcherOptions{Parallel: 2, ProjectRoot: "/p", WorktreeRoot: "/w", BaseBranch: "main", BinaryPath: "/bin/sf"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := NewDispatcher(tc.opts, nil)
			if tc.ok {
				assert.NoError(t, err)
				assert.NotNil(t, d)
				assert.Equal(t, "bmad", d.opts.Mode, "missing mode should default to bmad")
			} else {
				assert.Error(t, err)
			}
		})
	}
}
