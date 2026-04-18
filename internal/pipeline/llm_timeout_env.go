package pipeline

import (
	"os"
	"strings"
	"time"
)

// envLLMStepTimeout is the environment variable name for overriding the
// per-step Claude (LLM) subprocess timeout. Value is parsed with
// [time.ParseDuration] (e.g. "45m", "2h"). Empty or invalid values are ignored.
const envLLMStepTimeout = "BMAD_PIPELINE_STEP_TIMEOUT"

// LLMStepTimeoutFromEnv parses [envLLMStepTimeout]. Returns 0 if unset or
// invalid so callers keep the default [DefaultTimeout].
func LLMStepTimeoutFromEnv() time.Duration {
	raw := strings.TrimSpace(os.Getenv(envLLMStepTimeout))
	if raw == "" {
		return 0
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0
	}
	return d
}
