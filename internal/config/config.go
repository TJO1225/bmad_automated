package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/spf13/viper"
)

// Loader handles configuration loading from files and environment.
//
// Loader uses Viper to load configuration from YAML files and environment
// variables, merging them with default values. The loader supports the
// BMAD_ environment variable prefix for all configuration options.
type Loader struct {
	// v is the Viper instance used for configuration loading.
	v *viper.Viper
}

// NewLoader creates a new configuration loader.
//
// Returns a Loader ready to load configuration from files and environment.
// Call [Loader.Load] to perform the actual configuration loading.
func NewLoader() *Loader {
	return &Loader{
		v: viper.New(),
	}
}

// Load loads configuration from the default locations and environment.
//
// Configuration is loaded and merged with the following priority (highest first):
//  1. Environment variables with BMAD_ prefix (e.g., BMAD_CLAUDE_PATH)
//  2. Config file specified by BMAD_CONFIG_PATH environment variable
//  3. ./config/workflows.yaml in the current directory
//  4. [DefaultConfig] built-in defaults
//
// Environment variable names use underscores for nested keys. For example,
// claude.binary_path becomes BMAD_CLAUDE_BINARY_PATH.
//
// Returns an error if a config file exists but cannot be parsed. Missing
// config files are not an error; the loader falls back to defaults.
func (l *Loader) Load() (*Config, error) {
	// Start with defaults
	cfg := DefaultConfig()

	// Set up Viper
	l.v.SetConfigType("yaml")
	l.v.SetEnvPrefix("BMAD")
	l.v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	l.v.AutomaticEnv()

	// Try to find and read config file
	configPath := os.Getenv("BMAD_CONFIG_PATH")
	if configPath != "" {
		l.v.SetConfigFile(configPath)
	} else {
		// Look for config in current directory
		l.v.SetConfigName("workflows")
		l.v.AddConfigPath("./config")
		l.v.AddConfigPath(".")
	}

	// Read config file if it exists
	if err := l.v.ReadInConfig(); err != nil {
		// Config file not found is okay, we'll use defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			// Some other error occurred
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Unmarshal into config struct
	if err := l.v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	// Override Claude binary path from env if set
	if binaryPath := os.Getenv("BMAD_CLAUDE_PATH"); binaryPath != "" {
		cfg.Claude.BinaryPath = binaryPath
	}

	return cfg, nil
}

// LoadFromFile loads configuration from a specific file path.
//
// Unlike [Loader.Load], this method loads from an explicit file path without
// searching default locations or checking environment variables. The file
// extension determines the expected format (yaml, json, etc.).
//
// Returns an error if the file cannot be read or parsed.
func (l *Loader) LoadFromFile(path string) (*Config, error) {
	cfg := DefaultConfig()

	l.v.SetConfigFile(path)
	l.v.SetConfigType(filepath.Ext(path)[1:]) // Remove the dot

	if err := l.v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file %s: %w", path, err)
	}

	if err := l.v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}

	return cfg, nil
}

// GetPrompt returns the expanded prompt for a workflow and story key.
//
// The workflowName must match a key in the Workflows map. The storyKey is
// substituted into the workflow's prompt template using Go's text/template.
//
// Returns an error if the workflow is not found or if template expansion fails.
func (c *Config) GetPrompt(workflowName, storyKey string) (string, error) {
	workflow, ok := c.Workflows[workflowName]
	if !ok {
		return "", fmt.Errorf("unknown workflow: %s", workflowName)
	}

	return expandTemplate(workflow.PromptTemplate, PromptData{StoryKey: storyKey})
}

// GetModeSteps returns the ordered step names for a pipeline mode.
//
// Returns an error if the mode is not defined in [Config.Modes]. Callers
// should use the mode constants [ModeBmad] and [ModeBeads] to avoid typos.
func (c *Config) GetModeSteps(mode string) ([]string, error) {
	m, ok := c.Modes[mode]
	if !ok {
		return nil, fmt.Errorf("unknown mode: %s", mode)
	}
	return m.Steps, nil
}

// expandTemplate expands a Go template string with the given data.
func expandTemplate(tmpl string, data PromptData) (string, error) {
	t, err := template.New("prompt").Parse(tmpl)
	if err != nil {
		return "", fmt.Errorf("error parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("error executing template: %w", err)
	}

	return buf.String(), nil
}

// MustLoad loads configuration and panics on error.
//
// This is a convenience function for initialization code where configuration
// errors should be fatal. It creates a new [Loader] and calls [Loader.Load],
// panicking if an error occurs.
//
// Use this in main() or package initialization where there is no reasonable
// way to handle configuration errors.
func MustLoad() *Config {
	loader := NewLoader()
	cfg, err := loader.Load()
	if err != nil {
		panic(fmt.Sprintf("failed to load configuration: %v", err))
	}
	return cfg
}
