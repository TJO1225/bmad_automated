package status

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatus_IsValid(t *testing.T) {
	tests := []struct {
		name   string
		status Status
		want   bool
	}{
		{
			name:   "backlog is valid",
			status: StatusBacklog,
			want:   true,
		},
		{
			name:   "ready-for-dev is valid",
			status: StatusReadyForDev,
			want:   true,
		},
		{
			name:   "in-progress is valid",
			status: StatusInProgress,
			want:   true,
		},
		{
			name:   "review is valid",
			status: StatusReview,
			want:   true,
		},
		{
			name:   "done is valid",
			status: StatusDone,
			want:   true,
		},
		{
			name:   "empty string is invalid",
			status: Status(""),
			want:   false,
		},
		{
			name:   "unknown status is invalid",
			status: Status("unknown"),
			want:   false,
		},
		{
			name:   "typo in status is invalid",
			status: Status("in-progres"),
			want:   false,
		},
		{
			name:   "case sensitive - uppercase is invalid",
			status: Status("BACKLOG"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.status.IsValid()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStatus_Constants(t *testing.T) {
	assert.Equal(t, Status("backlog"), StatusBacklog)
	assert.Equal(t, Status("ready-for-dev"), StatusReadyForDev)
	assert.Equal(t, Status("in-progress"), StatusInProgress)
	assert.Equal(t, Status("review"), StatusReview)
	assert.Equal(t, Status("done"), StatusDone)
}

func TestEntryType_String(t *testing.T) {
	tests := []struct {
		name     string
		t        EntryType
		expected string
	}{
		{"epic", EntryTypeEpic, "epic"},
		{"story", EntryTypeStory, "story"},
		{"retrospective", EntryTypeRetrospective, "retrospective"},
		{"unknown", EntryType(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.t.String())
		})
	}
}
