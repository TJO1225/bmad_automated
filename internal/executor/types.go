package executor

// EventType represents the type of event received from an LLM's streaming output.
//
// Events flow through the stream in a typical order: system (init), then alternating
// assistant and user events, and finally a result event when the session completes.
type EventType string

const (
	// EventTypeSystem indicates a system event, typically session initialization.
	// Check [Event.SessionStarted] to detect the init subtype.
	EventTypeSystem EventType = "system"

	// EventTypeAssistant indicates output from the LLM, either text or tool invocations.
	// Use [Event.IsText] and [Event.IsToolUse] to distinguish between content types.
	EventTypeAssistant EventType = "assistant"

	// EventTypeUser indicates tool execution results returned to the LLM.
	// Use [Event.IsToolResult] to check if this event contains tool output.
	EventTypeUser EventType = "user"

	// EventTypeResult indicates the session has completed.
	// Check [Event.SessionComplete] which will be true for result events.
	EventTypeResult EventType = "result"
)

// SubtypeInit is the subtype value for system initialization events.
const SubtypeInit = "init"

// Event is a parsed event from an LLM's streaming output.
//
// This is the primary type that users interact with when processing LLM output.
// All backend parsers normalize their CLI-specific JSON into this shared type.
// Use the convenience methods [Event.IsText], [Event.IsToolUse], and
// [Event.IsToolResult] to quickly identify event types.
type Event struct {
	// Type is the parsed event type (system, assistant, user, or result).
	Type EventType

	// Subtype provides additional classification for certain event types.
	// For system events, this may be "init" (see [SubtypeInit]).
	Subtype string

	// Text contains the text content when Type is [EventTypeAssistant]
	// and the content block is of type "text". Empty otherwise.
	Text string

	// ToolName is the name of the tool being invoked when Type is
	// [EventTypeAssistant] and the content block is of type "tool_use".
	ToolName string

	// ToolDescription is a human-readable description of what the tool is doing.
	ToolDescription string

	// ToolCommand is the command string for bash/shell tool invocations.
	ToolCommand string

	// ToolFilePath is the file path for file operation tools.
	ToolFilePath string

	// ToolStdout contains the standard output from a tool execution.
	ToolStdout string

	// ToolStderr contains the standard error output from a tool execution.
	ToolStderr string

	// ToolInterrupted indicates whether tool execution was interrupted.
	ToolInterrupted bool

	// SessionStarted is true for system init events, indicating the
	// LLM session has begun.
	SessionStarted bool

	// SessionComplete is true for result events, indicating the
	// LLM session has finished.
	SessionComplete bool
}

// IsText returns true if this event contains text content from the LLM.
func (e Event) IsText() bool {
	return e.Type == EventTypeAssistant && e.Text != ""
}

// IsToolUse returns true if this event represents a tool invocation by the LLM.
func (e Event) IsToolUse() bool {
	return e.Type == EventTypeAssistant && e.ToolName != ""
}

// IsToolResult returns true if this event contains output from a tool execution.
func (e Event) IsToolResult() bool {
	return e.Type == EventTypeUser && (e.ToolStdout != "" || e.ToolStderr != "")
}
