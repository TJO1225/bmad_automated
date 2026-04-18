package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	// Check workflows exist (v6 BMAD slash-commands)
	assert.Contains(t, cfg.Workflows, "create-story")
	assert.Contains(t, cfg.Workflows, "dev-story")
	assert.Contains(t, cfg.Workflows, "code-review")

	// Check modes exist with expected step sequences
	assert.Contains(t, cfg.Modes, ModeBmad)
	assert.Contains(t, cfg.Modes, ModeBeads)
	assert.Equal(t, []string{"create-story", "dev-story", "code-review", "commit-branch", "open-pr", "review-pr"}, cfg.Modes[ModeBmad].Steps)
	assert.Equal(t, []string{"create-story", "sync-to-beads"}, cfg.Modes[ModeBeads].Steps)

	// Check defaults
	assert.Equal(t, "stream-json", cfg.Claude.OutputFormat)
	assert.Equal(t, "claude", cfg.Claude.BinaryPath)
	assert.Equal(t, 20, cfg.Output.TruncateLines)
	assert.Equal(t, 60, cfg.Output.TruncateLength)
}

func TestConfig_GetPrompt(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		name         string
		workflowName string
		storyKey     string
		wantContains string
		wantErr      bool
	}{
		{
			name:         "create-story",
			workflowName: "create-story",
			storyKey:     "test-123",
			wantContains: "test-123",
			wantErr:      false,
		},
		{
			name:         "dev-story",
			workflowName: "dev-story",
			storyKey:     "feature-456",
			wantContains: "feature-456",
			wantErr:      false,
		},
		{
			name:         "unknown workflow",
			workflowName: "unknown",
			storyKey:     "test",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prompt, err := cfg.GetPrompt(tt.workflowName, tt.storyKey)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, prompt, tt.wantContains)
			}
		})
	}
}

func TestConfig_GetModeSteps(t *testing.T) {
	cfg := DefaultConfig()

	bmad, err := cfg.GetModeSteps(ModeBmad)
	assert.NoError(t, err)
	assert.Equal(t, []string{"create-story", "dev-story", "code-review", "commit-branch", "open-pr", "review-pr"}, bmad)

	beads, err := cfg.GetModeSteps(ModeBeads)
	assert.NoError(t, err)
	assert.Equal(t, []string{"create-story", "sync-to-beads"}, beads)

	_, err = cfg.GetModeSteps("nonexistent")
	assert.Error(t, err)
}

func TestLoader_LoadFromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test-config.yaml")

	configContent := `
workflows:
  custom-workflow:
    prompt_template: "Custom: {{.StoryKey}}"
modes:
  custom:
    steps:
      - custom-workflow
claude:
  binary_path: /custom/path/claude
output:
  truncate_lines: 50
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	loader := NewLoader()
	cfg, err := loader.LoadFromFile(configPath)

	require.NoError(t, err)
	assert.Contains(t, cfg.Workflows, "custom-workflow")
	assert.Equal(t, []string{"custom-workflow"}, cfg.Modes["custom"].Steps)
	assert.Equal(t, "/custom/path/claude", cfg.Claude.BinaryPath)
	assert.Equal(t, 50, cfg.Output.TruncateLines)
}

func TestLoader_Load_WithEnvOverride(t *testing.T) {
	// Set environment variable
	os.Setenv("BMAD_CLAUDE_PATH", "/env/claude")
	defer os.Unsetenv("BMAD_CLAUDE_PATH")

	loader := NewLoader()
	cfg, err := loader.Load()

	require.NoError(t, err)
	assert.Equal(t, "/env/claude", cfg.Claude.BinaryPath)
}

func TestExpandTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     PromptData
		want     string
		wantErr  bool
	}{
		{
			name:     "simple substitution",
			template: "Story: {{.StoryKey}}",
			data:     PromptData{StoryKey: "test-123"},
			want:     "Story: test-123",
			wantErr:  false,
		},
		{
			name:     "multiple substitutions",
			template: "{{.StoryKey}} - {{.StoryKey}}",
			data:     PromptData{StoryKey: "abc"},
			want:     "abc - abc",
			wantErr:  false,
		},
		{
			name:     "no substitution",
			template: "Static text",
			data:     PromptData{StoryKey: "ignored"},
			want:     "Static text",
			wantErr:  false,
		},
		{
			name:     "invalid template",
			template: "{{.Invalid",
			data:     PromptData{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := expandTemplate(tt.template, tt.data)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestNewLoader(t *testing.T) {
	loader := NewLoader()
	assert.NotNil(t, loader)
	assert.NotNil(t, loader.v)
}

func TestLoader_LoadFromFile_NonExistent(t *testing.T) {
	loader := NewLoader()
	_, err := loader.LoadFromFile("/nonexistent/path/config.yaml")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error reading config file")
}

func TestLoader_LoadFromFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yaml")

	// Write invalid YAML
	invalidContent := `
workflows:
  - this is not valid yaml for this structure
    missing: colon here
`
	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	require.NoError(t, err)

	loader := NewLoader()
	_, err = loader.LoadFromFile(configPath)

	// Should error on unmarshal due to wrong structure
	assert.Error(t, err)
}

func TestLoader_Load_DefaultsWithNoConfigFile(t *testing.T) {
	// Ensure no config file exists in current dir
	// Load() should fall back to defaults
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Clear any env vars that might interfere
	os.Unsetenv("BMAD_CONFIG_PATH")
	os.Unsetenv("BMAD_CLAUDE_PATH")

	loader := NewLoader()
	cfg, err := loader.Load()

	require.NoError(t, err)
	assert.NotNil(t, cfg)
	// Should have default values
	assert.Equal(t, "claude", cfg.Claude.BinaryPath)
	assert.Equal(t, "stream-json", cfg.Claude.OutputFormat)
}

func TestLoader_Load_WithConfigPathEnv(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "custom-config.yaml")

	configContent := `
claude:
  binary_path: /from/env/path/claude
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	os.Setenv("BMAD_CONFIG_PATH", configPath)
	defer os.Unsetenv("BMAD_CONFIG_PATH")

	loader := NewLoader()
	cfg, err := loader.Load()

	require.NoError(t, err)
	assert.Equal(t, "/from/env/path/claude", cfg.Claude.BinaryPath)
}

func TestLoader_Load_EnvOverridesTakePrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Config file sets one path
	configContent := `
claude:
  binary_path: /from/file/claude
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	os.Setenv("BMAD_CONFIG_PATH", configPath)
	os.Setenv("BMAD_CLAUDE_PATH", "/from/env/override/claude")
	defer os.Unsetenv("BMAD_CONFIG_PATH")
	defer os.Unsetenv("BMAD_CLAUDE_PATH")

	loader := NewLoader()
	cfg, err := loader.Load()

	require.NoError(t, err)
	// Env var should take precedence
	assert.Equal(t, "/from/env/override/claude", cfg.Claude.BinaryPath)
}

func TestMustLoad_Success(t *testing.T) {
	// MustLoad should not panic when loading defaults
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	os.Unsetenv("BMAD_CONFIG_PATH")
	os.Unsetenv("BMAD_CLAUDE_PATH")

	// Should not panic
	cfg := MustLoad()
	assert.NotNil(t, cfg)
}

func TestConfig_GetPrompt_AllWorkflows(t *testing.T) {
	cfg := DefaultConfig()

	workflows := []string{"create-story", "dev-story", "code-review"}

	for _, wf := range workflows {
		t.Run(wf, func(t *testing.T) {
			prompt, err := cfg.GetPrompt(wf, "test-key")
			assert.NoError(t, err)
			assert.NotEmpty(t, prompt)
			assert.Contains(t, prompt, "test-key")
		})
	}
}

func TestLoader_LoadFromFile_DifferentExtension(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Write valid JSON config
	jsonContent := `{
		"claude": {
			"binary_path": "/json/path/claude"
		}
	}`
	err := os.WriteFile(configPath, []byte(jsonContent), 0644)
	require.NoError(t, err)

	loader := NewLoader()
	cfg, err := loader.LoadFromFile(configPath)

	require.NoError(t, err)
	assert.Equal(t, "/json/path/claude", cfg.Claude.BinaryPath)
}

func TestDefaultConfig_WorkflowTemplates(t *testing.T) {
	cfg := DefaultConfig()

	// Verify each workflow has a non-empty template
	for name, workflow := range cfg.Workflows {
		t.Run(name, func(t *testing.T) {
			assert.NotEmpty(t, workflow.PromptTemplate, "workflow %s should have a template", name)
		})
	}
}

func TestPromptData_StoryKey(t *testing.T) {
	data := PromptData{StoryKey: "ABC-123"}
	assert.Equal(t, "ABC-123", data.StoryKey)
}

func TestDefaultConfig_Backends(t *testing.T) {
	cfg := DefaultConfig()

	assert.Contains(t, cfg.Backends, BackendClaude)
	assert.Contains(t, cfg.Backends, BackendGemini)
	assert.Contains(t, cfg.Backends, BackendCursor)

	assert.Equal(t, "claude", cfg.Backends[BackendClaude].BinaryPath)
	assert.Equal(t, "gemini", cfg.Backends[BackendGemini].BinaryPath)
	assert.Equal(t, "cursor-agent", cfg.Backends[BackendCursor].BinaryPath)
	assert.Equal(t, "CURSOR_API_KEY", cfg.Backends[BackendCursor].APIKeyEnv)
}

func TestDefaultConfig_ModeDefaultBackend(t *testing.T) {
	cfg := DefaultConfig()

	assert.Equal(t, BackendClaude, cfg.Modes[ModeBmad].DefaultBackend)
	assert.Equal(t, BackendClaude, cfg.Modes[ModeBeads].DefaultBackend)
}

func TestGetPromptWithData_FallsBackToDefault(t *testing.T) {
	cfg := DefaultConfig()

	// No backend prompts, should use default template
	prompt, err := cfg.GetPromptWithData("create-story", "gemini", PromptData{StoryKey: "test-key"})
	require.NoError(t, err)
	assert.Contains(t, prompt, "test-key")
}

func TestGetPromptWithData_UsesBackendOverride(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workflows["review-pr"] = WorkflowConfig{
		PromptTemplate: "/code-review {{.PRURL}}",
		BackendPrompts: map[string]string{
			"gemini": "Review PR at {{.PRURL}} using gh pr diff. Story: {{.StoryKey}}",
		},
	}

	// Gemini backend should use override
	prompt, err := cfg.GetPromptWithData("review-pr", "gemini", PromptData{
		StoryKey: "1-2-test",
		PRURL:    "https://github.com/org/repo/pull/42",
	})
	require.NoError(t, err)
	assert.Contains(t, prompt, "gh pr diff")
	assert.Contains(t, prompt, "https://github.com/org/repo/pull/42")
	assert.Contains(t, prompt, "1-2-test")

	// Claude backend should fall back to default template
	prompt, err = cfg.GetPromptWithData("review-pr", "claude", PromptData{
		PRURL: "https://github.com/org/repo/pull/42",
	})
	require.NoError(t, err)
	assert.Contains(t, prompt, "/code-review")
	assert.Contains(t, prompt, "https://github.com/org/repo/pull/42")
}

func TestGetPromptWithData_EmptyBackendUsesDefault(t *testing.T) {
	cfg := DefaultConfig()

	prompt, err := cfg.GetPromptWithData("create-story", "", PromptData{StoryKey: "key"})
	require.NoError(t, err)
	assert.Contains(t, prompt, "key")
}

func TestGetPromptWithData_PRURL(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workflows["review-pr"] = WorkflowConfig{
		PromptTemplate: "Review {{.PRURL}} for story {{.StoryKey}}",
	}

	prompt, err := cfg.GetPromptWithData("review-pr", "", PromptData{
		StoryKey: "1-2-test",
		PRURL:    "https://github.com/org/repo/pull/99",
	})
	require.NoError(t, err)
	assert.Equal(t, "Review https://github.com/org/repo/pull/99 for story 1-2-test", prompt)
}

func TestLoader_Load_GeminiEnvOverride(t *testing.T) {
	os.Setenv("BMAD_GEMINI_PATH", "/custom/gemini")
	defer os.Unsetenv("BMAD_GEMINI_PATH")

	// Ensure no config file
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)
	os.Unsetenv("BMAD_CONFIG_PATH")
	os.Unsetenv("BMAD_CLAUDE_PATH")

	loader := NewLoader()
	cfg, err := loader.Load()
	require.NoError(t, err)
	assert.Equal(t, "/custom/gemini", cfg.Backends[BackendGemini].BinaryPath)
}
