package gemini

import (
	"encoding/json"
	"io"

	"story-factory/internal/executor"
)

// DefaultParser implements event parsing for Gemini's stream-json format.
//
// Gemini's JSON differs from Claude's in several ways:
//   - Session init uses {"type":"init"} (no subtype field)
//   - Text output uses {"type":"message","role":"assistant","content":"..."}
//   - Results use {"type":"result","status":"success"}
//
// The parser normalizes all of these into [executor.Event].
type DefaultParser struct {
	// BufferSize is the maximum size in bytes for a single JSON line.
	// Defaults to 10MB if not set or <= 0.
	BufferSize int
}

// NewParser creates a new [DefaultParser] with default settings.
func NewParser() *DefaultParser {
	return &DefaultParser{
		BufferSize: 10 * 1024 * 1024,
	}
}

// Parse reads streaming JSON from the reader and emits parsed [executor.Event] objects.
func (p *DefaultParser) Parse(reader io.Reader) <-chan executor.Event {
	events := make(chan executor.Event)

	scanner := executor.NewLineScanner()
	scanner.BufferSize = p.BufferSize

	go func() {
		defer close(events)

		for line := range scanner.Scan(reader) {
			var raw StreamEvent
			if err := json.Unmarshal([]byte(line), &raw); err != nil {
				continue
			}

			event := normalizeEvent(&raw)
			// Skip user echo messages (Gemini echoes the prompt back)
			if event.Type == "" {
				continue
			}
			events <- event
		}
	}()

	return events
}

// normalizeEvent converts a Gemini [StreamEvent] into a shared [executor.Event].
func normalizeEvent(raw *StreamEvent) executor.Event {
	switch raw.Type {
	case "init":
		return executor.Event{
			Type:           executor.EventTypeSystem,
			Subtype:        executor.SubtypeInit,
			SessionStarted: true,
		}

	case "message":
		return normalizeMessage(raw)

	case "tool_call":
		return executor.Event{
			Type:     executor.EventTypeAssistant,
			ToolName: raw.Name,
		}

	case "tool_result":
		return executor.Event{
			Type:            executor.EventTypeUser,
			ToolStdout:      raw.Stdout,
			ToolStderr:      raw.Stderr,
			ToolInterrupted: raw.Interrupted,
		}

	case "result":
		return executor.Event{
			Type:            executor.EventTypeResult,
			SessionComplete: true,
		}

	default:
		// Unknown event type — skip
		return executor.Event{}
	}
}

// normalizeMessage handles Gemini's {"type":"message"} events.
// The "role" field distinguishes user echo vs assistant output.
func normalizeMessage(raw *StreamEvent) executor.Event {
	switch raw.Role {
	case "assistant":
		return executor.Event{
			Type: executor.EventTypeAssistant,
			Text: raw.Content,
		}
	case "user":
		// Gemini echoes the user prompt — skip
		return executor.Event{}
	default:
		return executor.Event{}
	}
}

// ParseSingle parses a single JSON line into an [executor.Event].
func ParseSingle(line string) (executor.Event, error) {
	var raw StreamEvent
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return executor.Event{}, err
	}
	return normalizeEvent(&raw), nil
}
