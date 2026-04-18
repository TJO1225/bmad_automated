package gemini

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
type ExecutorConfig struct {
	// BinaryPath is the path to the Gemini CLI binary.
	// If empty, defaults to "gemini" which must be in PATH.
	BinaryPath string

	// OutputFormat is the output format flag value.
	// If empty, defaults to "stream-json".
	OutputFormat string

	// Parser is the JSON parser used to parse Gemini's streaming output.
	// If nil, a [DefaultParser] is created with default settings.
	Parser *DefaultParser

	// StderrHandler is called for each line written to stderr by Gemini.
	// If nil, stderr output is silently discarded.
	StderrHandler func(line string)

	// WorkingDir sets the working directory for the Gemini subprocess.
	// If empty, the subprocess inherits the parent process's working directory.
	WorkingDir string

	// GracePeriod controls graceful shutdown behavior when the context is canceled.
	GracePeriod time.Duration
}

// DefaultExecutor implements [executor.Executor] by spawning the Gemini CLI.
//
// Key differences from Claude:
//   - Prompt is positional (not -p flag)
//   - Auto-approve flag is --yolo (not --dangerously-skip-permissions)
//   - No --verbose flag
type DefaultExecutor struct {
	config ExecutorConfig
	parser *DefaultParser
}

// NewExecutor creates a new [DefaultExecutor] with the given configuration.
func NewExecutor(config ExecutorConfig) *DefaultExecutor {
	if config.BinaryPath == "" {
		config.BinaryPath = "gemini"
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

// Execute runs Gemini with the given prompt and returns a channel of events.
func (e *DefaultExecutor) Execute(ctx context.Context, prompt string) (<-chan executor.Event, error) {
	cmd := e.buildCommand(ctx, prompt)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start gemini: %w", err)
	}

	go e.handleStderr(stderr)

	events := e.parser.Parse(stdout)

	go func() {
		_ = cmd.Wait() //nolint:errcheck
	}()

	return events, nil
}

// ExecuteWithResult runs Gemini with the given prompt and waits for completion.
func (e *DefaultExecutor) ExecuteWithResult(ctx context.Context, prompt string, handler executor.EventHandler) (int, error) {
	cmd := e.buildCommand(ctx, prompt)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return 1, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return 1, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return 1, fmt.Errorf("failed to start gemini: %w", err)
	}

	go e.handleStderr(stderr)

	events := e.parser.Parse(stdout)
	for event := range events {
		if handler != nil {
			handler(event)
		}
	}

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

// buildCommand constructs the exec.Cmd for Gemini CLI.
//
// Gemini CLI uses positional prompt (not -p flag):
//
//	gemini --output-format stream-json --yolo "prompt text"
func (e *DefaultExecutor) buildCommand(ctx context.Context, prompt string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, e.config.BinaryPath,
		"--output-format", e.config.OutputFormat,
		"--yolo",
		prompt,
	)

	if e.config.WorkingDir != "" {
		cmd.Dir = e.config.WorkingDir
	}
	if e.config.GracePeriod > 0 {
		cmd.Cancel = func() error {
			return cmd.Process.Signal(syscall.SIGTERM)
		}
		cmd.WaitDelay = e.config.GracePeriod
	}

	return cmd
}

func (e *DefaultExecutor) handleStderr(stderr io.ReadCloser) {
	if e.config.StderrHandler == nil {
		_, _ = io.Copy(io.Discard, stderr) //nolint:errcheck
		return
	}

	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		e.config.StderrHandler(scanner.Text())
	}
}
