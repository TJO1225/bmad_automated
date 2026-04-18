// Package executor defines the shared interface for LLM CLI backends.
//
// This package provides the backend-agnostic types and interfaces that all
// LLM CLI integrations (Claude, Gemini, Cursor Agent) implement. The pipeline
// orchestrates steps through the [Executor] interface without knowing which
// backend is running.
//
// Backend implementations live in sibling packages:
//   - [story-factory/internal/claude] — Claude Code CLI
//   - [story-factory/internal/gemini] — Gemini CLI
//   - [story-factory/internal/cursor] — Cursor Agent CLI
package executor

import "context"

// Executor runs an LLM CLI and returns streaming events.
//
// Executor provides two execution modes:
//   - [Executor.Execute]: Fire-and-forget mode that returns a channel of [Event] objects.
//     Use this when you want to process events as they arrive but don't need the exit code.
//   - [Executor.ExecuteWithResult]: Blocking mode that processes events via an [EventHandler]
//     callback and returns the exit code. Use this for production workflows where you need
//     to know if the LLM completed successfully.
//
// For testing, use [MockExecutor] which implements this interface without spawning processes.
type Executor interface {
	// Execute runs the LLM with the given prompt and returns a channel of [Event] objects.
	// The channel is closed when the LLM exits or the context is canceled.
	// Returns an error if the LLM fails to start (e.g., binary not found).
	//
	// Note: This method is fire-and-forget; the exit status is not available.
	// Use [Executor.ExecuteWithResult] if you need the exit code.
	Execute(ctx context.Context, prompt string) (<-chan Event, error)

	// ExecuteWithResult runs the LLM with the given prompt and waits for completion.
	// The handler is called for each [Event] received during execution.
	// Returns the exit code (0 for success) and any error encountered during execution.
	//
	// This is the recommended method for production use as it provides the exit code
	// needed to determine if the LLM completed successfully.
	ExecuteWithResult(ctx context.Context, prompt string, handler EventHandler) (int, error)
}

// EventHandler is a callback function invoked for each [Event] received from the LLM.
//
// The handler is called synchronously in the order events are received. Handlers
// should process events quickly to avoid blocking the event stream.
type EventHandler func(event Event)

// Parser parses streaming JSON output from an LLM CLI.
//
// Each backend may produce different JSON formats, but all parsers normalize
// their output into the shared [Event] type. The channel returned by Parse is
// closed when EOF is reached or the underlying reader is closed.
type Parser interface {
	// Parse reads streaming JSON from the given reader and returns a channel of [Event] objects.
	// The channel is closed when the reader is exhausted or an error occurs.
	Parse(reader interface{ Read([]byte) (int, error) }) <-chan Event
}
