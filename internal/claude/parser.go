package claude

import (
	"encoding/json"
	"io"

	"story-factory/internal/executor"
)

// DefaultParser implements [executor.Parser] for Claude's stream-json format.
//
// DefaultParser uses a buffered scanner to read JSON lines from Claude's stdout.
// Each line is unmarshaled as a [StreamEvent] and normalized into an [executor.Event].
//
// Create instances using [NewParser] rather than constructing directly to ensure
// proper default values.
type DefaultParser struct {
	// BufferSize is the maximum size in bytes for a single JSON line.
	// Defaults to 10MB (10 * 1024 * 1024) if not set or <= 0.
	BufferSize int
}

// NewParser creates a new [DefaultParser] with default settings.
func NewParser() *DefaultParser {
	return &DefaultParser{
		BufferSize: 10 * 1024 * 1024,
	}
}

// Parse reads streaming JSON from the reader and emits parsed [executor.Event] objects.
//
// Error handling behavior:
//   - Empty lines are silently skipped
//   - Lines that fail JSON parsing are silently skipped
//   - Scanner errors terminate parsing and close the channel
//   - EOF closes the channel normally
func (p *DefaultParser) Parse(reader io.Reader) <-chan executor.Event {
	events := make(chan executor.Event)

	scanner := executor.NewLineScanner()
	scanner.BufferSize = p.BufferSize

	go func() {
		defer close(events)

		for line := range scanner.Scan(reader) {
			var streamEvent StreamEvent
			if err := json.Unmarshal([]byte(line), &streamEvent); err != nil {
				continue
			}
			events <- NewEventFromStream(&streamEvent)
		}
	}()

	return events
}

// ParseSingle parses a single JSON line into an [executor.Event].
//
// This is a utility function useful for testing and debugging.
func ParseSingle(line string) (executor.Event, error) {
	var streamEvent StreamEvent
	if err := json.Unmarshal([]byte(line), &streamEvent); err != nil {
		return executor.Event{}, err
	}
	return NewEventFromStream(&streamEvent), nil
}
