package cli

import (
	"time"

	"story-factory/internal/claude"
	"story-factory/internal/config"
	"story-factory/internal/cursor"
	"story-factory/internal/executor"
	"story-factory/internal/gemini"
)

// buildCommandExecutors constructs executors for all configured backends,
// setting the given working directory on each. This is called per-command
// because the working directory is resolved at runtime (from --project-dir
// or cwd).
//
// Returns the default executor (Claude) and a map of all named executors.
func buildCommandExecutors(cfg *config.Config, workingDir string) (executor.Executor, map[string]executor.Executor) {
	executors := make(map[string]executor.Executor)
	gracePeriod := 5 * time.Second

	// Always build Claude executor
	claudeCfg := cfg.Claude
	if b, ok := cfg.Backends[config.BackendClaude]; ok && b.BinaryPath != "" {
		claudeCfg.BinaryPath = b.BinaryPath
		if b.OutputFormat != "" {
			claudeCfg.OutputFormat = b.OutputFormat
		}
	}
	claudeExec := claude.NewExecutor(claude.ExecutorConfig{
		BinaryPath:   claudeCfg.BinaryPath,
		OutputFormat: claudeCfg.OutputFormat,
		WorkingDir:   workingDir,
		GracePeriod:  gracePeriod,
	})
	executors[config.BackendClaude] = claudeExec

	// Build Gemini executor if configured
	if b, ok := cfg.Backends[config.BackendGemini]; ok && b.BinaryPath != "" {
		executors[config.BackendGemini] = gemini.NewExecutor(gemini.ExecutorConfig{
			BinaryPath:   b.BinaryPath,
			OutputFormat: b.OutputFormat,
			WorkingDir:   workingDir,
			GracePeriod:  gracePeriod,
		})
	}

	// Build Cursor executor if configured
	if b, ok := cfg.Backends[config.BackendCursor]; ok && b.BinaryPath != "" {
		executors[config.BackendCursor] = cursor.NewExecutor(cursor.ExecutorConfig{
			BinaryPath:   b.BinaryPath,
			OutputFormat: b.OutputFormat,
			APIKeyEnv:    b.APIKeyEnv,
			WorkingDir:   workingDir,
			GracePeriod:  gracePeriod,
		})
	}

	return claudeExec, executors
}
