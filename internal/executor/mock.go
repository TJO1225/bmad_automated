package executor

import "context"

// MockExecutor implements [Executor] for testing without spawning real processes.
//
// Configure the mock by setting its fields before calling Execute or ExecuteWithResult:
//
//	mock := &MockExecutor{
//	    Events: []Event{{Type: EventTypeAssistant, Text: "Hello"}},
//	    ExitCode: 0,
//	}
//	exitCode, err := mock.ExecuteWithResult(ctx, "prompt", handler)
//
// After execution, check RecordedPrompts to verify the prompts that were passed:
//
//	if len(mock.RecordedPrompts) != 1 || mock.RecordedPrompts[0] != "expected prompt" {
//	    t.Error("unexpected prompt")
//	}
type MockExecutor struct {
	// Events is the list of [Event] objects to emit during execution.
	Events []Event

	// Error is returned from Execute/ExecuteWithResult if non-nil.
	// When set, no events are emitted.
	Error error

	// ExitCode is the value returned from [MockExecutor.ExecuteWithResult].
	ExitCode int

	// RecordedPrompts accumulates all prompts passed to Execute/ExecuteWithResult.
	RecordedPrompts []string
}

// Execute returns the pre-configured [MockExecutor.Events] via a channel.
func (m *MockExecutor) Execute(ctx context.Context, prompt string) (<-chan Event, error) {
	m.RecordedPrompts = append(m.RecordedPrompts, prompt)

	if m.Error != nil {
		return nil, m.Error
	}

	events := make(chan Event)
	go func() {
		defer close(events)
		for _, event := range m.Events {
			select {
			case <-ctx.Done():
				return
			case events <- event:
			}
		}
	}()

	return events, nil
}

// ExecuteWithResult returns the pre-configured [MockExecutor.ExitCode].
func (m *MockExecutor) ExecuteWithResult(ctx context.Context, prompt string, handler EventHandler) (int, error) {
	m.RecordedPrompts = append(m.RecordedPrompts, prompt)

	if m.Error != nil {
		return 1, m.Error
	}

	for _, event := range m.Events {
		if handler != nil {
			handler(event)
		}
	}

	return m.ExitCode, nil
}
