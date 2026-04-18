// Package claude provides the Claude Code CLI backend for story-factory.
//
// This package implements the [executor.Executor] interface by spawning
// Claude as a subprocess, parsing its streaming JSON output, and converting
// raw events into the shared [executor.Event] format.
//
// Claude-specific JSON types ([StreamEvent], [MessageContent], [ContentBlock],
// [ToolInput], [ToolResult]) live here because they map directly to Claude's
// stream-json wire format. The parser normalizes them into [executor.Event].
//
// For testing, use [executor.MockExecutor] which implements [executor.Executor]
// without spawning real processes.
package claude

import (
	"story-factory/internal/executor"
)

// Type aliases for backward compatibility. Code that previously imported
// these types from this package continues to compile. New code should
// import from [story-factory/internal/executor] directly.
type (
	Event        = executor.Event
	EventType    = executor.EventType
	EventHandler = executor.EventHandler
	Executor     = executor.Executor
	Parser       = executor.Parser
)

// Re-export constants for backward compatibility.
const (
	EventTypeSystem    = executor.EventTypeSystem
	EventTypeAssistant = executor.EventTypeAssistant
	EventTypeUser      = executor.EventTypeUser
	EventTypeResult    = executor.EventTypeResult
	SubtypeInit        = executor.SubtypeInit
)

// MockExecutor is an alias for [executor.MockExecutor].
type MockExecutor = executor.MockExecutor

// StreamEvent represents a raw JSON event from Claude's streaming output.
//
// This is the low-level structure that maps directly to Claude's stream-json format.
// The parser normalizes these into [executor.Event] via [NewEventFromStream].
type StreamEvent struct {
	Type          string          `json:"type"`
	Subtype       string          `json:"subtype,omitempty"`
	Message       *MessageContent `json:"message,omitempty"`
	ToolUseResult *ToolResult     `json:"tool_use_result,omitempty"`
}

// MessageContent represents the content of a message in Claude's streaming output.
type MessageContent struct {
	Content []ContentBlock `json:"content,omitempty"`
}

// ContentBlock represents a single block of content within a [MessageContent].
//
// The Type field indicates the kind of content:
//   - "text": Contains text output in the Text field
//   - "tool_use": Contains a tool invocation with Name and Input fields
type ContentBlock struct {
	Type  string     `json:"type"`
	Text  string     `json:"text,omitempty"`
	Name  string     `json:"name,omitempty"`
	Input *ToolInput `json:"input,omitempty"`
}

// ToolInput represents the input parameters for a tool invocation.
type ToolInput struct {
	Command     string `json:"command,omitempty"`
	Description string `json:"description,omitempty"`
	FilePath    string `json:"file_path,omitempty"`
	Content     string `json:"content,omitempty"`
}

// ToolResult represents the result of a tool execution in Claude's output.
type ToolResult struct {
	Stdout      string `json:"stdout,omitempty"`
	Stderr      string `json:"stderr,omitempty"`
	Interrupted bool   `json:"interrupted,omitempty"`
}

// NewEventFromStream creates an [executor.Event] from a Claude-specific [StreamEvent].
//
// This function normalizes Claude's JSON format into the shared event type.
func NewEventFromStream(raw *StreamEvent) Event {
	e := Event{
		Type:    EventType(raw.Type),
		Subtype: raw.Subtype,
	}

	switch e.Type {
	case EventTypeSystem:
		if raw.Subtype == SubtypeInit {
			e.SessionStarted = true
		}

	case EventTypeAssistant:
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

	case EventTypeUser:
		if raw.ToolUseResult != nil {
			e.ToolStdout = raw.ToolUseResult.Stdout
			e.ToolStderr = raw.ToolUseResult.Stderr
			e.ToolInterrupted = raw.ToolUseResult.Interrupted
		}

	case EventTypeResult:
		e.SessionComplete = true
	}

	return e
}
