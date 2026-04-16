// Package config provides configuration loading and management for story-factory.
//
// Configuration is loaded using Viper, supporting YAML config files and environment
// variable overrides. The package provides sensible defaults that work out of the
// box, with the ability to customize workflows, output formatting, and Claude CLI
// settings.
//
// Key types:
//   - [Config] is the root configuration container with all settings
//   - [Loader] handles Viper-based configuration loading
//   - [WorkflowConfig] defines a single workflow's prompt template
//   - [ModeConfig] defines an ordered sequence of steps for a pipeline mode
//   - [ClaudeConfig] contains Claude CLI binary settings
//
// Configuration priority (highest to lowest):
//  1. Environment variables (BMAD_ prefix)
//  2. Config file specified by BMAD_CONFIG_PATH
//  3. ./config/workflows.yaml
//  4. [DefaultConfig] defaults
package config

// Mode names recognized by the pipeline.
const (
	// ModeBmad runs the full BMAD v6 cycle: create-story -> dev-story ->
	// code-review (and, in Phase 2, commit-branch -> open-pr).
	ModeBmad = "bmad"

	// ModeBeads runs create-story -> sync-to-beads. Used when a project
	// tracks work in Gastown Beads rather than PRs.
	ModeBeads = "beads"
)

// Config represents the root configuration structure.
//
// This is the main configuration container loaded by [Loader] and used throughout
// the application. Use [DefaultConfig] to get sensible defaults.
type Config struct {
	// Workflows maps workflow names to their configurations.
	// Keys are workflow names (e.g., "create-story", "dev-story").
	Workflows map[string]WorkflowConfig `mapstructure:"workflows"`

	// Modes maps mode names (e.g., "bmad", "beads") to their step sequences.
	// Each mode declares which steps the pipeline runs in order.
	Modes map[string]ModeConfig `mapstructure:"modes"`

	// Claude contains Claude CLI binary configuration.
	Claude ClaudeConfig `mapstructure:"claude"`

	// Output contains terminal output formatting configuration.
	Output OutputConfig `mapstructure:"output"`
}

// WorkflowConfig represents a single workflow configuration.
//
// Each workflow has a prompt template that is expanded with story data
// using Go's text/template package.
type WorkflowConfig struct {
	// PromptTemplate is the Go template string for the workflow prompt.
	// Use {{.StoryKey}} to reference the story key.
	// Example: "Work on story: {{.StoryKey}}"
	PromptTemplate string `mapstructure:"prompt_template"`
}

// ModeConfig defines the ordered sequence of steps for a pipeline mode.
//
// Step names may reference either a Claude-driven workflow (with a template
// in [Config.Workflows]) or a built-in native step such as "commit-branch"
// or "open-pr". The pipeline resolves step names to executable functions via
// its step registry.
type ModeConfig struct {
	// Steps is the ordered list of step names to execute for this mode.
	Steps []string `mapstructure:"steps"`
}

// ClaudeConfig contains Claude CLI configuration.
//
// These settings control how the Claude CLI binary is invoked.
type ClaudeConfig struct {
	// OutputFormat is the output format passed to Claude CLI.
	// Should be "stream-json" for structured event parsing.
	OutputFormat string `mapstructure:"output_format"`

	// BinaryPath is the path to the Claude CLI binary.
	// Default: "claude" (assumes Claude is in PATH).
	// Can be overridden with BMAD_CLAUDE_PATH environment variable.
	BinaryPath string `mapstructure:"binary_path"`
}

// OutputConfig contains terminal output formatting configuration.
//
// These settings control how Claude's output is formatted in the terminal.
type OutputConfig struct {
	// TruncateLines is the maximum number of lines to display per event.
	// Additional lines are hidden with a "... (N more lines)" indicator.
	// Default: 20
	TruncateLines int `mapstructure:"truncate_lines"`

	// TruncateLength is the maximum length of each output line.
	// Longer lines are truncated with "..." suffix.
	// Default: 60
	TruncateLength int `mapstructure:"truncate_length"`
}

// DefaultConfig returns a new [Config] with sensible defaults.
//
// The defaults include v6-aligned BMAD slash-command workflows (create-story,
// dev-story, code-review) and two modes: "bmad" for the full BMAD cycle and
// "beads" for the create-and-sync-to-Beads flow. Native steps such as
// sync-to-beads, commit-branch, and open-pr are implemented in Go and do not
// need prompt templates.
func DefaultConfig() *Config {
	return &Config{
		Workflows: map[string]WorkflowConfig{
			"create-story": {
				PromptTemplate: "/bmad-create-story - Create story: {{.StoryKey}}. Do not ask questions.",
			},
			"dev-story": {
				PromptTemplate: "/bmad-dev-story - Implement story: {{.StoryKey}}. Complete every task in the story checklist, run tests as you go, and do not pause for confirmation. Use your best judgement based on existing patterns. Only stop when every task is checked and status is advanced to 'review'.",
			},
			"code-review": {
				PromptTemplate: "/bmad-code-review - Review story: {{.StoryKey}}. Review uncommitted changes in the working tree. Auto-apply any patches you propose rather than asking for confirmation.",
			},
		},
		Modes: map[string]ModeConfig{
			ModeBmad: {
				Steps: []string{"create-story", "dev-story", "code-review", "commit-branch", "open-pr"},
			},
			ModeBeads: {
				Steps: []string{"create-story", "sync-to-beads"},
			},
		},
		Claude: ClaudeConfig{
			OutputFormat: "stream-json",
			BinaryPath:   "claude",
		},
		Output: OutputConfig{
			TruncateLines:  20,
			TruncateLength: 60,
		},
	}
}

// PromptData contains data for workflow template expansion.
//
// This struct is passed to Go's text/template when expanding workflow prompts.
// Fields are accessible in templates using {{.FieldName}} syntax.
type PromptData struct {
	// StoryKey is the identifier of the story being processed.
	// Access in templates with {{.StoryKey}}.
	StoryKey string
}
