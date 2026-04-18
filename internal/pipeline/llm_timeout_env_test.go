package pipeline

import (
	"testing"
	"time"

	"story-factory/internal/config"
)

func TestLLMStepTimeoutFromEnv(t *testing.T) {
	t.Setenv(envLLMStepTimeout, "")
	if got := LLMStepTimeoutFromEnv(); got != 0 {
		t.Fatalf("empty env: got %v want 0", got)
	}

	t.Setenv(envLLMStepTimeout, "not-a-duration")
	if got := LLMStepTimeoutFromEnv(); got != 0 {
		t.Fatalf("invalid: got %v want 0", got)
	}

	t.Setenv(envLLMStepTimeout, "90m")
	if got := LLMStepTimeoutFromEnv(); got != 90*time.Minute {
		t.Fatalf("90m: got %v", got)
	}
}

func TestPipeline_llmTimeout(t *testing.T) {
	cfg := config.DefaultConfig()
	p := NewPipeline(nil, cfg, t.TempDir())
	if got := p.llmTimeout(); got != DefaultTimeout {
		t.Fatalf("default: got %v want %v", got, DefaultTimeout)
	}
	p2 := NewPipeline(nil, cfg, t.TempDir(), WithLLMStepTimeout(3*time.Hour))
	if got := p2.llmTimeout(); got != 3*time.Hour {
		t.Fatalf("override: got %v", got)
	}
}
