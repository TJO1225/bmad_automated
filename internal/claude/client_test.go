package claude

import (
	"context"
	"io"
	"testing"
	"time"

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

	ctx := context.Background()
	ch, err := mock.Execute(ctx, "test prompt")

	require.NoError(t, err)

	// Collect all events
	var collected []Event
	for event := range ch {
		collected = append(collected, event)
	}

	assert.Equal(t, events, collected)
	assert.Equal(t, []string{"test prompt"}, mock.RecordedPrompts)
}

func TestMockExecutor_Execute_WithError(t *testing.T) {
	mock := &MockExecutor{
		Error: assert.AnError,
	}

	ctx := context.Background()
	_, err := mock.Execute(ctx, "test prompt")

	assert.Error(t, err)
}

func TestMockExecutor_ExecuteWithResult(t *testing.T) {
	events := []Event{
		{Type: EventTypeSystem, SessionStarted: true},
		{Type: EventTypeAssistant, Text: "Hello!"},
		{Type: EventTypeResult, SessionComplete: true},
	}

	mock := &MockExecutor{
		Events:   events,
		ExitCode: 0,
	}

	var receivedEvents []Event
	handler := func(event Event) {
		receivedEvents = append(receivedEvents, event)
	}

	ctx := context.Background()
	exitCode, err := mock.ExecuteWithResult(ctx, "test prompt", handler)

	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Equal(t, events, receivedEvents)
}

func TestMockExecutor_ExecuteWithResult_NonZeroExit(t *testing.T) {
	mock := &MockExecutor{
		ExitCode: 1,
	}

	ctx := context.Background()
	exitCode, err := mock.ExecuteWithResult(ctx, "test prompt", nil)

	require.NoError(t, err)
	assert.Equal(t, 1, exitCode)
}

func TestMockExecutor_Execute_ContextCancellation(t *testing.T) {
	events := []Event{
		{Type: EventTypeSystem, SessionStarted: true},
		{Type: EventTypeAssistant, Text: "Hello!"},
		{Type: EventTypeResult, SessionComplete: true},
	}

	mock := &MockExecutor{Events: events}

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := mock.Execute(ctx, "test prompt")

	require.NoError(t, err)

	// Read first event
	<-ch

	// Cancel context
	cancel()

	// Should eventually close (may receive more events before closing)
	for range ch {
		// Drain channel
	}
}

func TestNewExecutor(t *testing.T) {
	// Test default config
	exec := NewExecutor(ExecutorConfig{})
	assert.NotNil(t, exec)
	assert.Equal(t, "claude", exec.config.BinaryPath)
	assert.Equal(t, "stream-json", exec.config.OutputFormat)
	assert.Equal(t, time.Duration(0), exec.config.GracePeriod)
	assert.Equal(t, "", exec.config.WorkingDir)

	// Test custom config
	exec = NewExecutor(ExecutorConfig{
		BinaryPath:   "/custom/claude",
		OutputFormat: "json",
		WorkingDir:   "/tmp/project",
		GracePeriod:  5 * time.Second,
	})
	assert.Equal(t, "/custom/claude", exec.config.BinaryPath)
	assert.Equal(t, "json", exec.config.OutputFormat)
	assert.Equal(t, "/tmp/project", exec.config.WorkingDir)
	assert.Equal(t, 5*time.Second, exec.config.GracePeriod)
}

func TestNewExecutor_WithCustomParser(t *testing.T) {
	customParser := NewParser()
	customParser.BufferSize = 5 * 1024 * 1024 // 5MB

	exec := NewExecutor(ExecutorConfig{
		Parser: customParser,
	})

	assert.NotNil(t, exec)
	assert.Equal(t, customParser, exec.parser)
}

func TestNewExecutor_WithStderrHandler(t *testing.T) {
	var capturedLines []string
	handler := func(line string) {
		capturedLines = append(capturedLines, line)
	}

	exec := NewExecutor(ExecutorConfig{
		StderrHandler: handler,
	})

	assert.NotNil(t, exec)
	assert.NotNil(t, exec.config.StderrHandler)
}

func TestMockExecutor_ExecuteWithResult_WithError(t *testing.T) {
	mock := &MockExecutor{
		Error: assert.AnError,
	}

	ctx := context.Background()
	exitCode, err := mock.ExecuteWithResult(ctx, "test prompt", nil)

	assert.Error(t, err)
	assert.Equal(t, 1, exitCode)
}

func TestMockExecutor_ExecuteWithResult_NilHandler(t *testing.T) {
	events := []Event{
		{Type: EventTypeSystem, SessionStarted: true},
		{Type: EventTypeAssistant, Text: "Hello!"},
		{Type: EventTypeResult, SessionComplete: true},
	}

	mock := &MockExecutor{
		Events:   events,
		ExitCode: 0,
	}

	ctx := context.Background()
	exitCode, err := mock.ExecuteWithResult(ctx, "test prompt", nil)

	require.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	// Should not panic with nil handler
}

func TestMockExecutor_MultiplePrompts(t *testing.T) {
	mock := &MockExecutor{
		Events: []Event{
			{Type: EventTypeResult, SessionComplete: true},
		},
		ExitCode: 0,
	}

	ctx := context.Background()

	// Execute multiple prompts
	_, _ = mock.Execute(ctx, "prompt 1")
	_, _ = mock.Execute(ctx, "prompt 2")
	_, _ = mock.ExecuteWithResult(ctx, "prompt 3", nil)

	assert.Equal(t, []string{"prompt 1", "prompt 2", "prompt 3"}, mock.RecordedPrompts)
}

// mockParser implements Parser for testing
type mockParser struct {
	events []Event
}

func (m *mockParser) Parse(reader io.Reader) <-chan Event {
	ch := make(chan Event)
	go func() {
		defer close(ch)
		for _, e := range m.events {
			ch <- e
		}
	}()
	return ch
}

func TestNewExecutor_UsesProvidedParser(t *testing.T) {
	customEvents := []Event{
		{Type: EventTypeSystem, SessionStarted: true},
	}
	customParser := &mockParser{events: customEvents}

	exec := NewExecutor(ExecutorConfig{
		Parser: customParser,
	})

	assert.Equal(t, customParser, exec.parser)
}
