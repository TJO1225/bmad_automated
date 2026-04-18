package gemini

import (
	"strings"
	"testing"

	"story-factory/internal/executor"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Real Gemini CLI output captured during research.
const realGeminiOutput = `{"type":"init","timestamp":"2026-04-16T20:47:28.760Z","session_id":"5f96698b-8636-437c-8ca0-c5694cedfdc9","model":"auto-gemini-3"}
{"type":"message","timestamp":"2026-04-16T20:47:28.761Z","role":"user","content":"Say hello\n\n\n"}
{"type":"message","timestamp":"2026-04-16T20:47:39.009Z","role":"assistant","content":"Hello Tom! I","delta":true}
{"type":"message","timestamp":"2026-04-16T20:47:39.083Z","role":"assistant","content":"'m ready to assist you with the bmad_automated project. Please provide your first command.","delta":true}
{"type":"result","timestamp":"2026-04-16T20:47:39.134Z","status":"success","stats":{"total_tokens":12123,"input_tokens":10764,"output_tokens":84,"cached":0,"input":10764,"duration_ms":10374,"tool_calls":0}}`

func TestDefaultParser_Parse_RealOutput(t *testing.T) {
	parser := NewParser()
	events := parser.Parse(strings.NewReader(realGeminiOutput))

	var collected []executor.Event
	for event := range events {
		collected = append(collected, event)
	}

	// Should have: init, assistant text x2, result (user message is skipped)
	require.Len(t, collected, 4)

	// Init event
	assert.Equal(t, executor.EventTypeSystem, collected[0].Type)
	assert.True(t, collected[0].SessionStarted)
	assert.Equal(t, executor.SubtypeInit, collected[0].Subtype)

	// First assistant delta
	assert.Equal(t, executor.EventTypeAssistant, collected[1].Type)
	assert.Equal(t, "Hello Tom! I", collected[1].Text)
	assert.True(t, collected[1].IsText())

	// Second assistant delta
	assert.Equal(t, executor.EventTypeAssistant, collected[2].Type)
	assert.Contains(t, collected[2].Text, "'m ready to assist")
	assert.True(t, collected[2].IsText())

	// Result event
	assert.Equal(t, executor.EventTypeResult, collected[3].Type)
	assert.True(t, collected[3].SessionComplete)
}

func TestDefaultParser_Parse_InitEvent(t *testing.T) {
	input := `{"type":"init","timestamp":"2026-04-16T20:47:28.760Z","session_id":"abc123","model":"gemini-3"}`

	parser := NewParser()
	events := parser.Parse(strings.NewReader(input))

	event := <-events
	assert.Equal(t, executor.EventTypeSystem, event.Type)
	assert.True(t, event.SessionStarted)
	assert.Equal(t, executor.SubtypeInit, event.Subtype)
}

func TestDefaultParser_Parse_SkipsUserEcho(t *testing.T) {
	input := `{"type":"message","role":"user","content":"test prompt"}
{"type":"message","role":"assistant","content":"response","delta":true}
{"type":"result","status":"success"}`

	parser := NewParser()
	events := parser.Parse(strings.NewReader(input))

	var collected []executor.Event
	for event := range events {
		collected = append(collected, event)
	}

	// User echo should be skipped, leaving assistant + result
	require.Len(t, collected, 2)
	assert.Equal(t, executor.EventTypeAssistant, collected[0].Type)
	assert.Equal(t, "response", collected[0].Text)
	assert.Equal(t, executor.EventTypeResult, collected[1].Type)
}

func TestDefaultParser_Parse_ResultEvent(t *testing.T) {
	input := `{"type":"result","timestamp":"2026-04-16T20:47:39.134Z","status":"success","stats":{"total_tokens":100}}`

	parser := NewParser()
	events := parser.Parse(strings.NewReader(input))

	event := <-events
	assert.Equal(t, executor.EventTypeResult, event.Type)
	assert.True(t, event.SessionComplete)
}

func TestDefaultParser_Parse_SkipsInvalidJSON(t *testing.T) {
	input := `{"type":"init"}
not valid json
{"type":"result","status":"success"}`

	parser := NewParser()
	events := parser.Parse(strings.NewReader(input))

	var collected []executor.Event
	for event := range events {
		collected = append(collected, event)
	}

	require.Len(t, collected, 2)
	assert.Equal(t, executor.EventTypeSystem, collected[0].Type)
	assert.Equal(t, executor.EventTypeResult, collected[1].Type)
}

func TestDefaultParser_Parse_EmptyLines(t *testing.T) {
	input := `{"type":"init"}

{"type":"result","status":"success"}
`

	parser := NewParser()
	events := parser.Parse(strings.NewReader(input))

	var collected []executor.Event
	for event := range events {
		collected = append(collected, event)
	}

	require.Len(t, collected, 2)
}

func TestParseSingle(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
		check   func(t *testing.T, event executor.Event)
	}{
		{
			name:    "init event",
			input:   `{"type":"init","model":"gemini-3"}`,
			wantErr: false,
			check: func(t *testing.T, event executor.Event) {
				assert.Equal(t, executor.EventTypeSystem, event.Type)
				assert.True(t, event.SessionStarted)
			},
		},
		{
			name:    "assistant message",
			input:   `{"type":"message","role":"assistant","content":"hello","delta":true}`,
			wantErr: false,
			check: func(t *testing.T, event executor.Event) {
				assert.Equal(t, executor.EventTypeAssistant, event.Type)
				assert.Equal(t, "hello", event.Text)
			},
		},
		{
			name:    "result event",
			input:   `{"type":"result","status":"success"}`,
			wantErr: false,
			check: func(t *testing.T, event executor.Event) {
				assert.Equal(t, executor.EventTypeResult, event.Type)
				assert.True(t, event.SessionComplete)
			},
		},
		{
			name:    "invalid json",
			input:   `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := ParseSingle(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.check != nil {
					tt.check(t, event)
				}
			}
		})
	}
}

func TestNewExecutor(t *testing.T) {
	exec := NewExecutor(ExecutorConfig{})
	assert.NotNil(t, exec)
	assert.Equal(t, "gemini", exec.config.BinaryPath)
	assert.Equal(t, "stream-json", exec.config.OutputFormat)

	exec = NewExecutor(ExecutorConfig{
		BinaryPath:   "/custom/gemini",
		OutputFormat: "json",
		WorkingDir:   "/tmp/project",
	})
	assert.Equal(t, "/custom/gemini", exec.config.BinaryPath)
	assert.Equal(t, "json", exec.config.OutputFormat)
	assert.Equal(t, "/tmp/project", exec.config.WorkingDir)
}
