package cursor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"story-factory/internal/executor"
)

// ExecutorConfig contains configuration for creating a [DefaultExecutor].
type ExecutorConfig struct {
	// BinaryPath is the path to the cursor-agent binary.
	// If empty, defaults to "cursor-agent" which must be in PATH.
	BinaryPath string

	// OutputFormat is the output format flag value.
	// If empty, defaults to "stream-json".
	OutputFormat string

	// APIKeyEnv is the name of the environment variable containing the API key.
	// If empty, defaults to "CURSOR_API_KEY".
	// The executor reads the key from this env var at execution time.
	APIKeyEnv string

	// Parser is the JSON parser used to parse Cursor Agent's streaming output.
	// If nil, a [DefaultParser] is created with default settings.
	Parser *DefaultParser

	// StderrHandler is called for each line written to stderr.
	StderrHandler func(line string)

	// WorkingDir sets the working directory for the subprocess.
	WorkingDir string

	// GracePeriod controls graceful shutdown behavior.
	GracePeriod time.Duration
}

// DefaultExecutor implements [executor.Executor] by spawning cursor-agent.
//
// cursor-agent uses:
//   - -p flag for print/non-interactive mode
//   - --output-format stream-json for structured output
//   - --force to auto-approve all tool calls
//   - Positional prompt after flags
//   - Optional --api-key for authentication
type DefaultExecutor struct {
	config ExecutorConfig
	parser *DefaultParser
}

// NewExecutor creates a new [DefaultExecutor] with the given configuration.
func NewExecutor(config ExecutorConfig) *DefaultExecutor {
	if config.BinaryPath == "" {
		config.BinaryPath = "cursor-agent"
	}
	if config.OutputFormat == "" {
		config.OutputFormat = "stream-json"
	}
	if config.APIKeyEnv == "" {
		config.APIKeyEnv = "CURSOR_API_KEY"
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

// Execute runs cursor-agent with the given prompt and returns a channel of events.
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
		return nil, fmt.Errorf("failed to start cursor-agent: %w", err)
	}

	go e.handleStderr(stderr)

	events := e.parser.Parse(stdout)

	go func() {
		_ = cmd.Wait() //nolint:errcheck
	}()

	return events, nil
}

// ExecuteWithResult runs cursor-agent with the given prompt and waits for completion.
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
		return 1, fmt.Errorf("failed to start cursor-agent: %w", err)
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

// buildCommand constructs the exec.Cmd for cursor-agent.
//
// cursor-agent flags:
//
//	cursor-agent -p --output-format stream-json --force [--api-key KEY] "prompt"
func (e *DefaultExecutor) buildCommand(ctx context.Context, prompt string) *exec.Cmd {
	args := []string{
		"-p",
		"--output-format", e.config.OutputFormat,
		"--force",
	}

	// Add API key if available from environment
	if apiKey := os.Getenv(e.config.APIKeyEnv); apiKey != "" {
		args = append(args, "--api-key", apiKey)
	}

	// Prompt is positional (after flags)
	args = append(args, prompt)

	cmd := exec.CommandContext(ctx, e.config.BinaryPath, args...)

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
