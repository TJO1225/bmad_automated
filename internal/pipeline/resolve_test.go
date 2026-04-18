package pipeline

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"story-factory/internal/config"
	"story-factory/internal/executor"
)

func TestResolveExecutor_WorkflowOverride(t *testing.T) {
	defaultExec := &executor.MockExecutor{}
	geminiExec := &executor.MockExecutor{}

	cfg := config.DefaultConfig()
	cfg.Workflows["code-review"] = config.WorkflowConfig{
		PromptTemplate: "review {{.StoryKey}}",
		Backend:        "gemini",
	}

	p := NewPipeline(defaultExec, cfg, "/tmp",
		WithExecutors(map[string]executor.Executor{
			"claude": defaultExec,
			"gemini": geminiExec,
		}),
	)

	// code-review specifies backend: gemini
	got := p.resolveExecutor("code-review")
	assert.Equal(t, geminiExec, got)

	// create-story has no override, falls back to mode default
	got = p.resolveExecutor("create-story")
	assert.Equal(t, defaultExec, got)
}

func TestResolveExecutor_ModeDefault(t *testing.T) {
	defaultExec := &executor.MockExecutor{}
	geminiExec := &executor.MockExecutor{}

	cfg := config.DefaultConfig()
	cfg.Modes["bmad"] = config.ModeConfig{
		Steps:          []string{"create-story", "dev-story"},
		DefaultBackend: "gemini",
	}

	p := NewPipeline(defaultExec, cfg, "/tmp",
		WithExecutors(map[string]executor.Executor{
			"gemini": geminiExec,
		}),
	)

	// No workflow override; mode default is gemini
	got := p.resolveExecutor("create-story")
	assert.Equal(t, geminiExec, got)
}

func TestResolveExecutor_FallsBackToDefault(t *testing.T) {
	defaultExec := &executor.MockExecutor{}

	cfg := config.DefaultConfig()
	// No workflow overrides, no mode default backend set
	cfg.Modes["bmad"] = config.ModeConfig{
		Steps: []string{"create-story"},
	}

	p := NewPipeline(defaultExec, cfg, "/tmp")

	got := p.resolveExecutor("create-story")
	assert.Equal(t, defaultExec, got)
}

func TestResolveExecutor_UnknownBackendFallsBack(t *testing.T) {
	defaultExec := &executor.MockExecutor{}

	cfg := config.DefaultConfig()
	cfg.Workflows["dev-story"] = config.WorkflowConfig{
		PromptTemplate: "dev {{.StoryKey}}",
		Backend:        "nonexistent",
	}

	p := NewPipeline(defaultExec, cfg, "/tmp")

	// Backend "nonexistent" not in executors map, falls through
	got := p.resolveExecutor("dev-story")
	assert.Equal(t, defaultExec, got)
}

func TestResolveBackendName(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Workflows["review-pr"] = config.WorkflowConfig{
		PromptTemplate: "review",
		Backend:        "gemini",
	}
	cfg.Modes["bmad"] = config.ModeConfig{
		Steps:          []string{"create-story", "review-pr"},
		DefaultBackend: "claude",
	}

	p := NewPipeline(&executor.MockExecutor{}, cfg, "/tmp")

	// Workflow override takes priority
	assert.Equal(t, "gemini", p.resolveBackendName("review-pr"))

	// No workflow override; falls to mode default
	assert.Equal(t, "claude", p.resolveBackendName("create-story"))

	// Unknown workflow; falls to mode default
	assert.Equal(t, "claude", p.resolveBackendName("unknown-step"))
}
