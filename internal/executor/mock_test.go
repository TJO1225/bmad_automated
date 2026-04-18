package executor

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMockExecutor_Execute(t *testing.T) {
	events := []Event{
		{Type: EventTypeSystem, SessionStarted: true},
		{Type: EventTypeAssistant, Text: "Hello!"},
		{Type: EventTypeResult, SessionComplete: true},
	}

	mock := &MockExecutor{Events: events}

	ch, err := mock.Execute(context.Background(), "test prompt")
	require.NoError(t, err)

	var collected []Event
	for event := range ch {
		collected = append(collected, event)
	}

	assert.Equal(t, events, collected)
	assert.Equal(t, []string{"test prompt"}, mock.RecordedPrompts)
}

func TestMockExecutor_ExecuteWithResult(t *testing.T) {
	events := []Event{
		{Type: EventTypeAssistant, Text: "Working..."},
	}

	mock := &MockExecutor{Events: events, ExitCode: 0}

	var received []Event
	exitCode, err := mock.ExecuteWithResult(context.Background(), "prompt", func(e Event) {
		received = append(received, e)
	})

	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, events, received)
}

func TestMockExecutor_Error(t *testing.T) {
	mock := &MockExecutor{Error: errors.New("binary not found")}

	_, err := mock.Execute(context.Background(), "prompt")
	assert.Error(t, err)

	code, err := mock.ExecuteWithResult(context.Background(), "prompt", nil)
	assert.Error(t, err)
	assert.Equal(t, 1, code)
}

func TestEvent_IsText(t *testing.T) {
	assert.True(t, Event{Type: EventTypeAssistant, Text: "hi"}.IsText())
	assert.False(t, Event{Type: EventTypeAssistant}.IsText())
	assert.False(t, Event{Type: EventTypeSystem, Text: "hi"}.IsText())
}

func TestEvent_IsToolUse(t *testing.T) {
	assert.True(t, Event{Type: EventTypeAssistant, ToolName: "Bash"}.IsToolUse())
	assert.False(t, Event{Type: EventTypeAssistant}.IsToolUse())
}

func TestEvent_IsToolResult(t *testing.T) {
	assert.True(t, Event{Type: EventTypeUser, ToolStdout: "out"}.IsToolResult())
	assert.True(t, Event{Type: EventTypeUser, ToolStderr: "err"}.IsToolResult())
	assert.False(t, Event{Type: EventTypeUser}.IsToolResult())
}

func TestLineScanner_Scan(t *testing.T) {
	// LineScanner is tested via the parsers, but verify basic behavior
	scanner := NewLineScanner()
	assert.Equal(t, 10*1024*1024, scanner.BufferSize)
}
