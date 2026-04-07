package output

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewPrinter(t *testing.T) {
	p := NewPrinter()
	assert.NotNil(t, p)
}

func TestNewPrinterWithWriter(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)
	assert.NotNil(t, p)
}

func TestDefaultPrinter_SessionStart(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.SessionStart()

	output := buf.String()
	assert.Contains(t, output, "Session started")
}

func TestDefaultPrinter_SessionEnd(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.SessionEnd(5*time.Second, true)

	output := buf.String()
	assert.Contains(t, output, "Session complete")
	assert.Contains(t, output, "5s")
}

func TestDefaultPrinter_StepEnd(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.StepEnd(1500*time.Millisecond, true)

	output := buf.String()
	assert.Contains(t, output, "Step done")
	assert.Contains(t, output, "1.5s")
}

func TestDefaultPrinter_StepStart(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.StepStart(1, 4, "create-story")

	output := buf.String()
	assert.Contains(t, output, "[1/4]")
	assert.Contains(t, output, "create-story")
}

func TestDefaultPrinter_ToolUse(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.ToolUse("Bash", "List files", "ls -la", "")

	output := buf.String()
	assert.Contains(t, output, "Bash")
	assert.Contains(t, output, "List files")
	assert.Contains(t, output, "ls -la")
}

func TestDefaultPrinter_ToolUse_WithFilePath(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.ToolUse("Read", "", "", "/path/to/file.go")

	output := buf.String()
	assert.Contains(t, output, "Read")
	assert.Contains(t, output, "/path/to/file.go")
}

func TestDefaultPrinter_ToolResult(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.ToolResult("file1.go\nfile2.go", "", 20)

	output := buf.String()
	assert.Contains(t, output, "file1.go")
	assert.Contains(t, output, "file2.go")
}

func TestDefaultPrinter_ToolResult_WithStderr(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.ToolResult("", "error message", 20)

	output := buf.String()
	assert.Contains(t, output, "stderr")
	assert.Contains(t, output, "error message")
}

func TestDefaultPrinter_Text(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.Text("Hello from Claude!")

	output := buf.String()
	assert.Contains(t, output, "Claude:")
	assert.Contains(t, output, "Hello from Claude!")
}

func TestDefaultPrinter_Text_Empty(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.Text("")

	output := buf.String()
	assert.Empty(t, output)
}

func TestDefaultPrinter_CommandHeader(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.CommandHeader("create-story: test-123", "Long prompt here", 20)

	output := buf.String()
	assert.Contains(t, output, "create-story: test-123")
}

func TestDefaultPrinter_CommandFooter_Success(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.CommandFooter(5*time.Second, true, 0)

	output := buf.String()
	assert.Contains(t, output, "SUCCESS")
}

func TestDefaultPrinter_CommandFooter_Failure(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.CommandFooter(5*time.Second, false, 1)

	output := buf.String()
	assert.Contains(t, output, "FAILED")
	assert.Contains(t, output, "Exit code: 1")
}

func TestDefaultPrinter_CycleHeader(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.CycleHeader("test-story")

	output := buf.String()
	assert.Contains(t, output, "BMAD Full Cycle")
	assert.Contains(t, output, "test-story")
}

func TestDefaultPrinter_CycleSummary(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	steps := []StepResult{
		{Name: "create-story", Duration: 10 * time.Second, Success: true},
		{Name: "dev-story", Duration: 30 * time.Second, Success: true},
	}

	p.CycleSummary("test-story", steps, 40*time.Second)

	output := buf.String()
	assert.Contains(t, output, "CYCLE COMPLETE")
	assert.Contains(t, output, "test-story")
	assert.Contains(t, output, "create-story")
	assert.Contains(t, output, "dev-story")
}

func TestDefaultPrinter_CycleFailed(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.CycleFailed("test-story", "dev-story", 15*time.Second)

	output := buf.String()
	assert.Contains(t, output, "CYCLE FAILED")
	assert.Contains(t, output, "test-story")
	assert.Contains(t, output, "dev-story")
}

func TestDefaultPrinter_QueueHeader(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.QueueHeader(3, []string{"story-1", "story-2", "story-3"})

	output := buf.String()
	assert.Contains(t, output, "BMAD Queue")
	assert.Contains(t, output, "3 stories")
}

func TestDefaultPrinter_QueueStoryStart(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	p.QueueStoryStart(2, 5, "story-key")

	output := buf.String()
	assert.Contains(t, output, "QUEUE")
	assert.Contains(t, output, "[2/5]")
	assert.Contains(t, output, "story-key")
}

func TestDefaultPrinter_QueueSummary_AllSuccess(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	results := []StoryResult{
		{Key: "1-1-foo", Success: true, Duration: 10 * time.Second, ValidationLoops: 1, BeadID: "abc123"},
		{Key: "1-2-bar", Success: true, Duration: 20 * time.Second, ValidationLoops: 2, BeadID: "def456"},
	}
	counts := BatchCounts{Created: 2, Validated: 2, Synced: 2}

	p.QueueSummary(results, counts, 30*time.Second)

	out := buf.String()
	assert.Contains(t, out, "BATCH COMPLETE")
	assert.Contains(t, out, "loops:1")
	assert.Contains(t, out, "bead:abc123")
	assert.Contains(t, out, "loops:2")
	assert.Contains(t, out, "bead:def456")
	assert.Contains(t, out, "Created: 2")
	assert.Contains(t, out, "Validated: 2")
	assert.Contains(t, out, "Synced: 2")
	assert.Contains(t, out, "Failed: 0")
}

func TestDefaultPrinter_QueueSummary_MixedBatch(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	results := []StoryResult{
		{Key: "1-1-foo", Success: true, Duration: 12 * time.Second, ValidationLoops: 2, BeadID: "abc123"},
		{Key: "1-2-bar", FailedAt: "validate", Reason: "timed out", Duration: 18 * time.Second},
		{Key: "1-3-baz", Skipped: true},
	}
	counts := BatchCounts{Created: 1, Validated: 1, Synced: 1, Failed: 1, Skipped: 1}

	p.QueueSummary(results, counts, 30*time.Second)

	out := buf.String()
	assert.Contains(t, out, "BATCH FAILED")
	assert.Contains(t, out, "loops:2")
	assert.Contains(t, out, "bead:abc123")
	assert.Contains(t, out, "validate")
	assert.Contains(t, out, "timed out")
	assert.Contains(t, out, "skipped")
	assert.Contains(t, out, "Created: 1")
	assert.Contains(t, out, "Failed: 1")
	assert.Contains(t, out, "Skip: 1")
}

func TestDefaultPrinter_QueueSummary_AllFailed(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	results := []StoryResult{
		{Key: "1-1-foo", FailedAt: "create", Reason: "not found", Duration: 5 * time.Second},
		{Key: "1-2-bar", FailedAt: "validate", Reason: "diverged", Duration: 8 * time.Second},
	}
	counts := BatchCounts{Failed: 2}

	p.QueueSummary(results, counts, 13*time.Second)

	out := buf.String()
	assert.Contains(t, out, "BATCH FAILED")
	assert.Contains(t, out, "create")
	assert.Contains(t, out, "not found")
	assert.Contains(t, out, "validate")
	assert.Contains(t, out, "diverged")
	assert.Contains(t, out, "Failed: 2")
	assert.Contains(t, out, "Created: 0")
}

func TestDefaultPrinter_QueueSummary_AllSkipped(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	results := []StoryResult{
		{Key: "1-1-foo", Skipped: true},
		{Key: "1-2-bar", Skipped: true},
	}
	counts := BatchCounts{Skipped: 2}

	p.QueueSummary(results, counts, 1*time.Second)

	out := buf.String()
	assert.Contains(t, out, "BATCH COMPLETE")
	assert.Contains(t, out, "skipped")
	assert.Contains(t, out, "Skip: 2")
	assert.Contains(t, out, "Created: 0")
}

func TestDefaultPrinter_BatchSummary_EpicGrouped(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	results := []StoryResult{
		{Key: "1-1-scaffold", Success: true, Duration: 8 * time.Second, ValidationLoops: 1, BeadID: "aaa"},
		{Key: "1-2-schema", Success: true, Duration: 12 * time.Second, ValidationLoops: 1, BeadID: "bbb"},
		{Key: "2-1-preconditions", Success: true, Duration: 10 * time.Second, ValidationLoops: 1, BeadID: "ccc"},
		{Key: "2-2-creation", FailedAt: "create", Reason: "file not found", Duration: 15 * time.Second},
	}
	counts := BatchCounts{Created: 3, Validated: 3, Synced: 3, Failed: 1}

	p.BatchSummary(results, counts, 45*time.Second)

	out := buf.String()
	assert.Contains(t, out, "QUEUE FAILED")
	assert.Contains(t, out, "Epic 1")
	assert.Contains(t, out, "Epic 2")
	assert.Contains(t, out, "bead:aaa")
	assert.Contains(t, out, "bead:bbb")
	assert.Contains(t, out, "bead:ccc")
	assert.Contains(t, out, "file not found")
	assert.Contains(t, out, "2 created, 0 failed")
	assert.Contains(t, out, "1 created, 1 failed")
	assert.Contains(t, out, "Created: 3")
	assert.Contains(t, out, "Failed: 1")
}

func TestDefaultPrinter_BatchSummary_AllSuccess(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	results := []StoryResult{
		{Key: "1-1-a", Success: true, Duration: 5 * time.Second, ValidationLoops: 1, BeadID: "x1"},
		{Key: "2-1-b", Success: true, Duration: 7 * time.Second, ValidationLoops: 1, BeadID: "x2"},
	}
	counts := BatchCounts{Created: 2, Validated: 2, Synced: 2}

	p.BatchSummary(results, counts, 12*time.Second)

	out := buf.String()
	assert.Contains(t, out, "QUEUE COMPLETE")
	assert.Contains(t, out, "Epic 1")
	assert.Contains(t, out, "Epic 2")
	assert.Contains(t, out, "Created: 2")
}

func TestDefaultPrinter_BatchSummary_WithSkipped(t *testing.T) {
	var buf bytes.Buffer
	p := NewPrinterWithWriter(&buf)

	results := []StoryResult{
		{Key: "1-1-a", Success: true, Duration: 5 * time.Second, ValidationLoops: 1, BeadID: "x1"},
		{Key: "1-2-b", Skipped: true},
		{Key: "2-1-c", FailedAt: "sync", Reason: "bd not found", Duration: 10 * time.Second},
	}
	counts := BatchCounts{Created: 1, Validated: 1, Synced: 1, Failed: 1, Skipped: 1}

	p.BatchSummary(results, counts, 15*time.Second)

	out := buf.String()
	assert.Contains(t, out, "QUEUE FAILED")
	assert.Contains(t, out, "skipped")
	assert.Contains(t, out, "bd not found")
	assert.Contains(t, out, "Skip: 1")
	assert.Contains(t, out, "Failed: 1")
	assert.Contains(t, out, "1 skipped")
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is a long string", 10, "this is..."},
		{"", 10, ""},
	}

	for _, tt := range tests {
		result := truncateString(tt.input, tt.maxLen)
		assert.Equal(t, tt.expected, result)
	}
}

func TestTruncateOutput(t *testing.T) {
	// Create 30 lines
	lines := make([]string, 30)
	for i := range lines {
		lines[i] = "line"
	}
	input := strings.Join(lines, "\n")

	result := truncateOutput(input, 10)

	assert.Contains(t, result, "lines omitted")
}

func TestTruncateOutput_NoTruncation(t *testing.T) {
	input := "line1\nline2\nline3"
	result := truncateOutput(input, 10)

	assert.Equal(t, input, result)
}

func TestTruncateOutput_ZeroMaxLines(t *testing.T) {
	input := "line1\nline2\nline3"
	result := truncateOutput(input, 0)

	assert.Equal(t, input, result)
}
