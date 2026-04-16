package beads

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "standard heading",
			input: "# Story 1.2: Database Schema\n\nSome content",
			want:  "Database Schema",
		},
		{
			name:  "multi-word title with high story number",
			input: "# Story 2.10: Complex Multi-Word Title\n",
			want:  "Complex Multi-Word Title",
		},
		{
			name:  "title with extra whitespace",
			input: "# Story 1.1:   Spaced Title  \n",
			want:  "Spaced Title",
		},
		{
			name:  "heading not on first line",
			input: "Some preamble\n\n# Story 3.1: Later Heading\n",
			want:  "Later Heading",
		},
		{
			name:    "no H1 heading at all",
			input:   "## Not a Story Heading\n\nSome content",
			wantErr: "no title heading found",
		},
		{
			name:  "heading without colon falls back to full H1",
			input: "# Story 1.2 Missing Colon\n",
			want:  "Story 1.2 Missing Colon",
		},
		{
			name:    "empty content",
			input:   "",
			wantErr: "no title heading found",
		},
		{
			name:  "non-standard H1 falls back to raw heading text",
			input: "# Epic 12 Phase 5a Cleanup\n\nSome body\n",
			want:  "Epic 12 Phase 5a Cleanup",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractTitle(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractAcceptanceCriteria(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "standard section between headings",
			input: "## Acceptance Criteria\n\n1. First criterion\n2. Second criterion\n\n## Tasks / Subtasks\n\n- [ ] Task 1\n",
			want:  "1. First criterion\n2. Second criterion",
		},
		{
			name:  "section at EOF",
			input: "## Story\n\nAs a user...\n\n## Acceptance Criteria\n\n1. Given something\n   When action\n   Then result\n",
			want:  "1. Given something\n   When action\n   Then result",
		},
		{
			name:    "missing section",
			input:   "## Story\n\nSome content\n\n## Tasks\n\n- [ ] Task 1\n",
			wantErr: "no '## Acceptance Criteria' section found",
		},
		{
			name:    "empty section",
			input:   "## Acceptance Criteria\n\n## Tasks / Subtasks\n",
			wantErr: "acceptance criteria section is empty",
		},
		{
			name:    "empty content",
			input:   "",
			wantErr: "no '## Acceptance Criteria' section found",
		},
		{
			name:  "section with multiple paragraphs",
			input: "## Acceptance Criteria\n\nParagraph one.\n\nParagraph two.\n\n## Dev Notes\n",
			want:  "Paragraph one.\n\nParagraph two.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractAcceptanceCriteria(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractTitle_FromTestdataFixtures(t *testing.T) {
	tests := []struct {
		fixture string
		want    string
	}{
		{"story_valid.md", "Database Schema"},
		{"story_minimal.md", "Complex Multi-Word Title"},
		{"story_ac_at_eof.md", "Final Section Story"},
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata", tt.fixture))
			require.NoError(t, err)

			got, err := ExtractTitle(string(content))
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtractAcceptanceCriteria_FromTestdataFixtures(t *testing.T) {
	tests := []struct {
		fixture string
		wantErr bool
	}{
		{"story_valid.md", false},
		{"story_minimal.md", true}, // no AC section
		{"story_ac_at_eof.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			content, err := os.ReadFile(filepath.Join("testdata", tt.fixture))
			require.NoError(t, err)

			got, err := ExtractAcceptanceCriteria(string(content))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, got)
		})
	}
}
