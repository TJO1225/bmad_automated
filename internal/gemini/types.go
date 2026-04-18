// Package gemini provides the Gemini CLI backend for story-factory.
//
// This package implements the [executor.Executor] interface by spawning
// the Gemini CLI as a subprocess. Gemini's stream-json format differs from
// Claude's: it uses {"type":"message","role":"assistant"} instead of
// {"type":"assistant","message":{...}}.
//
// The parser normalizes Gemini's JSON into the shared [executor.Event] type
// so the pipeline can treat all backends uniformly.
package gemini

// StreamEvent represents a raw JSON event from Gemini's streaming output.
//
// Gemini's stream-json format uses a flat structure:
//   - Init:      {"type":"init","timestamp":"...","session_id":"...","model":"..."}
//   - User msg:  {"type":"message","role":"user","content":"..."}
//   - Assistant:  {"type":"message","role":"assistant","content":"...","delta":true}
//   - Tool use:  {"type":"tool_call","name":"...","args":{...}} (TBD — needs real capture)
//   - Tool result:{"type":"tool_result","name":"...","stdout":"...","stderr":"..."} (TBD)
//   - Result:    {"type":"result","status":"success","stats":{...}}
type StreamEvent struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype,omitempty"`
	Role      string `json:"role,omitempty"`
	Content   string `json:"content,omitempty"`
	Delta     bool   `json:"delta,omitempty"`
	Status    string `json:"status,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Model     string `json:"model,omitempty"`

	// Tool use fields (provisional — captured format TBD)
	Name string      `json:"name,omitempty"`
	Args interface{} `json:"args,omitempty"`

	// Tool result fields
	Stdout      string `json:"stdout,omitempty"`
	Stderr      string `json:"stderr,omitempty"`
	Interrupted bool   `json:"interrupted,omitempty"`

	// Stats for result events
	Stats *ResultStats `json:"stats,omitempty"`
}

// ResultStats contains token usage and timing information from Gemini.
type ResultStats struct {
	TotalTokens  int `json:"total_tokens,omitempty"`
	InputTokens  int `json:"input_tokens,omitempty"`
	OutputTokens int `json:"output_tokens,omitempty"`
	Cached       int `json:"cached,omitempty"`
	DurationMs   int `json:"duration_ms,omitempty"`
	ToolCalls    int `json:"tool_calls,omitempty"`
}
