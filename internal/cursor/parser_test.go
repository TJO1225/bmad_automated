package cursor

import (
	"strings"
	"testing"

	"story-factory/internal/executor"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// PROVISIONAL: These tests assume Claude-compatible JSON format.
// They will be updated once cursor-agent is authenticated and real output is captured.

func TestDefaultParser_Parse_ClaudeCompatible(t *testing.T) {
	input := `{"type":"system","subtype":"init"}
{"type":"assistant","message":{"content":[{"type":"text","text":"Hello!"}]}}
{"type":"result"}`

	parser := NewParser()
	events := parser.Parse(strings.NewReader(input))

	var collected []executor.Event
	for event := range events {
		collected = append(collected, event)
	}

	require.Len(t, collected, 3)

	assert.Equal(t, executor.EventTypeSystem, collected[0].Type)
	assert.True(t, collected[0].SessionStarted)

	assert.Equal(t, executor.EventTypeAssistant, collected[1].Type)
	assert.Equal(t, "Hello!", collected[1].Text)

	assert.Equal(t, executor.EventTypeResult, collected[2].Type)
	assert.True(t, collected[2].SessionComplete)
}

func TestDefaultParser_Parse_ToolUse(t *testing.T) {
	input := `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"ls","description":"List files"}}]}}`

	parser := NewParser()
	events := parser.Parse(strings.NewReader(input))

	event := <-events
	assert.Equal(t, "Bash", event.ToolName)
	assert.Equal(t, "ls", event.ToolCommand)
	assert.Equal(t, "List files", event.ToolDescription)
}

func TestDefaultParser_Parse_ToolResult(t *testing.T) {
	input := `{"type":"user","tool_use_result":{"stdout":"output","stderr":""}}`

	parser := NewParser()
	events := parser.Parse(strings.NewReader(input))

	event := <-events
	assert.Equal(t, executor.EventTypeUser, event.Type)
	assert.Equal(t, "output", event.ToolStdout)
	assert.True(t, event.IsToolResult())
}

func TestDefaultParser_Parse_SkipsInvalidJSON(t *testing.T) {
	input := `{"type":"system","subtype":"init"}
not valid json
{"type":"result"}`

	parser := NewParser()
	events := parser.Parse(strings.NewReader(input))

	var collected []executor.Event
	for event := range events {
		collected = append(collected, event)
	}

	require.Len(t, collected, 2)
}

func TestParseSingle(t *testing.T) {
	event, err := ParseSingle(`{"type":"system","subtype":"init"}`)
	require.NoError(t, err)
	assert.Equal(t, executor.EventTypeSystem, event.Type)
	assert.True(t, event.SessionStarted)
}

func TestNewExecutor(t *testing.T) {
	exec := NewExecutor(ExecutorConfig{})
	assert.NotNil(t, exec)
	assert.Equal(t, "cursor-agent", exec.config.BinaryPath)
	assert.Equal(t, "stream-json", exec.config.OutputFormat)
	assert.Equal(t, "CURSOR_API_KEY", exec.config.APIKeyEnv)

	exec = NewExecutor(ExecutorConfig{
		BinaryPath: "/custom/cursor-agent",
		APIKeyEnv:  "MY_CURSOR_KEY",
	})
	assert.Equal(t, "/custom/cursor-agent", exec.config.BinaryPath)
	assert.Equal(t, "MY_CURSOR_KEY", exec.config.APIKeyEnv)
}
