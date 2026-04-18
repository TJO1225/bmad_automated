package cursor

import (
	"encoding/json"
	"io"

	"story-factory/internal/executor"
)

// StreamEvent represents a raw JSON event from Cursor Agent's streaming output.
//
// PROVISIONAL: This assumes Claude-compatible JSON format. Cursor Agent's
// stream-json output has not been captured yet (auth required). The structure
// will be refined when real output is available.
type StreamEvent struct {
	Type          string          `json:"type"`
	Subtype       string          `json:"subtype,omitempty"`
	Message       *MessageContent `json:"message,omitempty"`
	ToolUseResult *ToolResult     `json:"tool_use_result,omitempty"`
}

// MessageContent represents the content of a message.
type MessageContent struct {
	Content []ContentBlock `json:"content,omitempty"`
}

// ContentBlock represents a single block of content.
type ContentBlock struct {
	Type  string     `json:"type"`
	Text  string     `json:"text,omitempty"`
	Name  string     `json:"name,omitempty"`
	Input *ToolInput `json:"input,omitempty"`
}

// ToolInput represents tool invocation parameters.
type ToolInput struct {
	Command     string `json:"command,omitempty"`
	Description string `json:"description,omitempty"`
	FilePath    string `json:"file_path,omitempty"`
	Content     string `json:"content,omitempty"`
}

// ToolResult represents tool execution output.
type ToolResult struct {
	Stdout      string `json:"stdout,omitempty"`
	Stderr      string `json:"stderr,omitempty"`
	Interrupted bool   `json:"interrupted,omitempty"`
}

// DefaultParser implements event parsing for Cursor Agent's stream-json format.
//
// PROVISIONAL: Currently assumes Claude-compatible JSON. Will be updated
// once cursor-agent is authenticated and real output is captured.
type DefaultParser struct {
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
			events <- normalizeEvent(&raw)
		}
	}()

	return events
}

// normalizeEvent converts a Cursor Agent [StreamEvent] into a shared [executor.Event].
// Currently mirrors Claude's normalization logic.
func normalizeEvent(raw *StreamEvent) executor.Event {
	e := executor.Event{
		Type:    executor.EventType(raw.Type),
		Subtype: raw.Subtype,
	}

	switch e.Type {
	case executor.EventTypeSystem:
		if raw.Subtype == executor.SubtypeInit {
			e.SessionStarted = true
		}

	case executor.EventTypeAssistant:
		if raw.Message != nil {
			for _, block := range raw.Message.Content {
				switch block.Type {
				case "text":
					e.Text = block.Text
				case "tool_use":
					e.ToolName = block.Name
					if block.Input != nil {
						e.ToolDescription = block.Input.Description
						e.ToolCommand = block.Input.Command
						e.ToolFilePath = block.Input.FilePath
					}
				}
			}
		}

	case executor.EventTypeUser:
		if raw.ToolUseResult != nil {
			e.ToolStdout = raw.ToolUseResult.Stdout
			e.ToolStderr = raw.ToolUseResult.Stderr
			e.ToolInterrupted = raw.ToolUseResult.Interrupted
		}

	case executor.EventTypeResult:
		e.SessionComplete = true
	}

	return e
}

// ParseSingle parses a single JSON line into an [executor.Event].
func ParseSingle(line string) (executor.Event, error) {
	var raw StreamEvent
	if err := json.Unmarshal([]byte(line), &raw); err != nil {
		return executor.Event{}, err
	}
	return normalizeEvent(&raw), nil
}
