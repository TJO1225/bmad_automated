package claude

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"

	"story-factory/internal/executor"
)

// ExecutorConfig contains configuration for creating a [DefaultExecutor].
//
// All fields are optional and have sensible defaults. Use [NewExecutor] to create
// an executor with this configuration.
type ExecutorConfig struct {
	// BinaryPath is the path to the Claude CLI binary.
	// If empty, defaults to "claude" which must be in PATH.
	BinaryPath string

	// OutputFormat is the Claude CLI output format flag.
	// If empty, defaults to "stream-json" which is required for event parsing.
	OutputFormat string

	// Parser is the JSON parser used to parse Claude's streaming output.
	// If nil, a [DefaultParser] is created with default settings.
	Parser *DefaultParser

	// StderrHandler is called for each line written to stderr by Claude.
	// If nil, stderr output is silently discarded.
	StderrHandler func(line string)

	// WorkingDir sets the working directory for the Claude subprocess.
	// If empty, the subprocess inherits the parent process's working directory.
	WorkingDir string

	// GracePeriod controls graceful shutdown behavior when the context is canceled.
	// When > 0, context cancellation sends SIGTERM and waits GracePeriod before
	// force-killing the process. When 0 (default), context cancellation sends
	// SIGKILL immediately (Go's default behavior).
	GracePeriod time.Duration
}

// DefaultExecutor implements [executor.Executor] by spawning Claude as a subprocess.
//
// This is the production implementation that uses os/exec to run the Claude CLI.
// It captures stdout for event parsing and optionally handles stderr via the
// configured [ExecutorConfig.StderrHandler].
//
// Create instances using [NewExecutor] rather than constructing directly.
type DefaultExecutor struct {
	config ExecutorConfig
	parser *DefaultParser
}

// NewExecutor creates a new [DefaultExecutor] with the given configuration.
//
// Default values are applied for any unset configuration fields:
//   - BinaryPath defaults to "claude"
//   - OutputFormat defaults to "stream-json"
//   - Parser defaults to a new [DefaultParser]
func NewExecutor(config ExecutorConfig) *DefaultExecutor {
	if config.BinaryPath == "" {
		config.BinaryPath = "claude"
	}
	if config.OutputFormat == "" {
		config.OutputFormat = "stream-json"
	}

	parser := config.Parser
	if parser == nil {
		parser = NewParser()
	}

	return &DefaultExecutor{
		config: config,
		parser: parser,
	}
}

// Execute runs Claude with the given prompt and returns a channel of [executor.Event] objects.
//
// The returned channel emits events as they are parsed from Claude's streaming output.
// The channel is closed when Claude exits, the context is canceled, or an error occurs.
//
// Note: This method does not provide the exit status. Use [DefaultExecutor.ExecuteWithResult]
// if you need to check whether Claude completed successfully.
func (e *DefaultExecutor) Execute(ctx context.Context, prompt string) (<-chan executor.Event, error) {
	cmd := exec.CommandContext(ctx, e.config.BinaryPath,
		"-p", prompt,
		"--output-format", e.config.OutputFormat,
		"--verbose",
		"--dangerously-skip-permissions",
	)
	e.applyCommandConfig(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start claude: %w", err)
	}

	// Handle stderr in background
	go e.handleStderr(stderr)

	// Parse stdout and return events channel
	events := e.parser.Parse(stdout)

	// Wait for command completion in background.
	go func() {
		_ = cmd.Wait() //nolint:errcheck // Exit status intentionally ignored; use ExecuteWithResult if needed
	}()

	return events, nil
}

// ExecuteWithResult runs Claude with the given prompt and waits for completion.
//
// This is the recommended method for production use. It processes events via the
// provided [executor.EventHandler] callback and returns the exit code when Claude completes.
//
// Exit code semantics:
//   - 0: Claude completed successfully
//   - Non-zero: Claude exited with an error
func (e *DefaultExecutor) ExecuteWithResult(ctx context.Context, prompt string, handler executor.EventHandler) (int, error) {
	cmd := exec.CommandContext(ctx, e.config.BinaryPath,
		"-p", prompt,
		"--output-format", e.config.OutputFormat,
		"--verbose",
		"--dangerously-skip-permissions",
	)
	e.applyCommandConfig(cmd)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 1, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("failed to start claude: %w", err)
	}

	// Handle stderr in background
	go e.handleStderr(stderr)

	// Process events
	events := e.parser.Parse(stdout)
	for event := range events {
		if handler != nil {
			handler(event)
		}
	}

	// Wait for command completion
	err = cmd.Wait()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return 1, err
		}
	}

	return exitCode, nil
}

// applyCommandConfig sets WorkingDir and GracePeriod on the command
// before Start() is called.
func (e *DefaultExecutor) applyCommandConfig(cmd *exec.Cmd) {
	if e.config.WorkingDir != "" {
		cmd.Dir = e.config.WorkingDir
	}
	if e.config.GracePeriod > 0 {
		cmd.Cancel = func() error {
			return cmd.Process.Signal(syscall.SIGTERM)
		}
		cmd.WaitDelay = e.config.GracePeriod
	}
}

func (e *DefaultExecutor) handleStderr(stderr io.ReadCloser) {
	if e.config.StderrHandler == nil {
		_, _ = io.Copy(io.Discard, stderr) //nolint:errcheck // Intentionally discarding stderr
		return
	}

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		e.config.StderrHandler(scanner.Text())
	}
}
