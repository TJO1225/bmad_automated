// Package cli provides the command-line interface for story-factory.
//
// The cli package implements Cobra-based commands for orchestrating
// automated development workflows. It uses dependency injection via the
// [App] struct to wire up all required services, enabling comprehensive
// testing without subprocess execution.
//
// Key types:
//   - [App] - Main application container with injected dependencies
//   - [StatusReader] - Interface for reading story status from sprint-status.yaml
//   - [ExecuteResult] - Result type returned by testable entry points
package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"story-factory/internal/beads"
	"story-factory/internal/claude"
	"story-factory/internal/config"
	"story-factory/internal/output"
	"story-factory/internal/status"
)

// StatusReader is the interface for reading story status from sprint-status.yaml.
//
// The production implementation is [status.Reader], which parses the YAML
// file at _bmad-output/implementation-artifacts/sprint-status.yaml.
type StatusReader interface {
	// BacklogStories returns all story entries with backlog status,
	// sorted by epic number then story number.
	BacklogStories() ([]status.Entry, error)

	// StoriesByStatus returns all story entries matching the given status,
	// sorted by epic number then story number.
	StoriesByStatus(status string) ([]status.Entry, error)

	// StoriesForEpic returns all story entries for the given epic number,
	// sorted by story number.
	StoriesForEpic(n int) ([]status.Entry, error)

	// StoryByKey returns the entry matching the given key, or status.ErrStoryNotFound.
	StoryByKey(key string) (*status.Entry, error)

	// ResolveStoryLocation resolves the story_location path template from
	// sprint-status.yaml by replacing {project-root} with the given project directory.
	ResolveStoryLocation(projectDir string) (string, error)
}

// App is the main application container with dependency injection.
//
// All dependencies are injected via struct fields, enabling comprehensive
// testing by substituting mock implementations. The production constructor
// [NewApp] wires up real implementations; tests can construct App directly
// with mock dependencies.
type App struct {
	// Config holds application configuration including workflow definitions.
	Config *config.Config

	// Executor runs Claude CLI as a subprocess and streams JSON events.
	Executor claude.Executor

	// Printer formats and displays output to the terminal.
	Printer output.Printer

	// StatusReader reads story status from sprint-status.yaml.
	StatusReader StatusReader

	// BeadsExecutor runs the bd CLI to create beads.
	BeadsExecutor beads.Executor

	// CheckPreconditions, if set, replaces [pipeline.CheckAll] in [App.RunPreconditions].
	// Tests use this to skip environment-specific checks (for example bd CLI on PATH).
	CheckPreconditions func(projectDir string) error

	// DryRun shows planned operations without executing subprocesses.
	DryRun bool

	// Verbose streams Claude CLI output in real time.
	Verbose bool

	// ProjectDir overrides the project root directory (default: current working directory).
	ProjectDir string
}

// NewApp creates a new [App] with all production dependencies wired up.
//
// This constructor initializes:
//   - A [claude.Executor] configured from cfg.Claude settings
//   - A [status.Reader] for sprint status management
//   - An [output.Printer] for terminal output
//
// For testing, construct [App] directly with mock dependencies instead.
func NewApp(cfg *config.Config) *App {
	printer := output.NewPrinter()

	executor := claude.NewExecutor(claude.ExecutorConfig{
		BinaryPath:   cfg.Claude.BinaryPath,
		OutputFormat: cfg.Claude.OutputFormat,
		GracePeriod:  5 * time.Second,
		StderrHandler: func(line string) {
			os.Stderr.WriteString("[stderr] " + line + "\n")
		},
	})

	statusReader := status.NewReader("")

	return &App{
		Config:        cfg,
		Executor:      executor,
		Printer:       printer,
		StatusReader:  statusReader,
		BeadsExecutor: beads.NewExecutor(),
	}
}

// ResolveProjectDir returns the project directory from the --project-dir flag,
// falling back to [os.Getwd] if the flag was not set.
//
// This must be called inside a command's RunE (after flag parsing), not during
// App construction.
func (app *App) ResolveProjectDir() (string, error) {
	if trimmed := strings.TrimSpace(app.ProjectDir); trimmed != "" {
		return trimmed, nil
	}
	return os.Getwd()
}

// NewRootCommand creates the root Cobra command and registers subcommands
// (create-story, validate-story, run, epic, queue).
func NewRootCommand(app *App) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "story-factory",
		Short: "Story Factory CLI",
		Long: `Story Factory CLI - Automate story processing pipelines with Claude.

This tool orchestrates Claude to run story processing pipelines including
story creation, validation, development, and review.`,
	}

	rootCmd.PersistentFlags().BoolVar(&app.DryRun, "dry-run", false, "Show planned operations without executing")
	rootCmd.PersistentFlags().BoolVar(&app.Verbose, "verbose", false, "Stream Claude CLI output in real time")
	rootCmd.PersistentFlags().StringVar(&app.ProjectDir, "project-dir", "", "Project root directory (default: current working directory)")

	rootCmd.AddCommand(NewCreateStoryCommand(app))
	rootCmd.AddCommand(NewValidateStoryCommand(app))
	rootCmd.AddCommand(newRunCommand(app))
	rootCmd.AddCommand(newEpicCommand(app))
	rootCmd.AddCommand(newQueueCommand(app))

	return rootCmd
}

// ExecuteResult holds the result of running the CLI.
//
// This type enables testable CLI execution by returning exit codes and errors
// instead of calling os.Exit() directly. Use [Run] or [RunWithConfig] to get
// an ExecuteResult; [Execute] handles os.Exit() internally.
type ExecuteResult struct {
	// ExitCode is the exit code to return to the shell (0 = success).
	ExitCode int

	// Err is the error that caused a non-zero exit code, if any.
	Err error
}

// RunWithConfig creates the app and executes the root command with a pre-loaded config.
//
// This is the testable core of [Execute], accepting an already-loaded [config.Config]
// so tests can provide custom configurations. It creates an [App] via [NewApp],
// builds the command tree via [NewRootCommand], and executes the command.
//
// Exit codes:
//   - 0: Success
//   - 1: Config or command error
//   - Non-zero from subprocess: Passed through from Claude CLI
func RunWithConfig(cfg *config.Config) ExecuteResult {
	app := NewApp(cfg)
	rootCmd := NewRootCommand(app)

	if err := rootCmd.Execute(); err != nil {
		if code, ok := IsExitError(err); ok {
			return ExecuteResult{ExitCode: code, Err: err}
		}
		return ExecuteResult{ExitCode: 1, Err: err}
	}
	return ExecuteResult{ExitCode: 0, Err: nil}
}

// Run loads configuration and executes the CLI, returning the result.
//
// This is the fully testable entry point that:
//  1. Loads configuration via [config.NewLoader]
//  2. Calls [RunWithConfig] with the loaded config
//
// Use this for integration tests that need to test config loading.
// For unit tests with custom configs, use [RunWithConfig] directly.
func Run() ExecuteResult {
	cfg, err := config.NewLoader().Load()
	if err != nil {
		return ExecuteResult{
			ExitCode: 1,
			Err:      fmt.Errorf("error loading config: %w", err),
		}
	}
	return RunWithConfig(cfg)
}

// Execute runs the CLI application and exits the process.
//
// This is the entry point called by main(). It calls [Run] and translates
// the [ExecuteResult] into an os.Exit() call. Because it exits the process,
// this function is not testable; use [Run] or [RunWithConfig] for tests.
func Execute() {
	result := Run()
	if result.ExitCode != 0 {
		os.Exit(result.ExitCode)
	}
}
