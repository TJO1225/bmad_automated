# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Build & Development Commands

```bash
just build              # Build binary to ./story-factory
just test               # Run all tests
just test-verbose       # Run tests with verbose output
just test-pkg ./internal/claude  # Test specific package
just test-coverage      # Generate coverage.html
just lint               # Run golangci-lint
just check              # Run fmt, vet, and test
just run --help         # Build and run with arguments
```

## Architecture

This is a CLI tool that orchestrates Claude CLI to run automated story processing pipelines. It spawns Claude as a subprocess, parses its streaming JSON output, and displays formatted results.

### Package Dependencies

```text
cmd/story-factory/main.go
         │
         ▼
    internal/cli (root command only, no subcommands yet)
         │
         ├──► internal/claude (Claude execution + JSON parsing)
         ├──► internal/config (Viper configuration)
         ├──► internal/output (terminal formatting)
         └──► internal/status (sprint status reader)
```

### Key Interfaces for Testing

- **`claude.Executor`** - Interface for running Claude CLI. Use `MockExecutor` in tests to avoid spawning real processes.
- **`output.Printer`** - Interface for terminal output. Use `NewPrinterWithWriter(buf)` to capture output in tests.

### Data Flow

1. CLI command receives story key
2. `config.Config.GetPrompt()` expands Go template with `{{.StoryKey}}`
3. `claude.Executor.ExecuteWithResult()` runs Claude CLI
4. `claude.Parser` reads streaming JSON, emits `Event` structs
5. `output.Printer` formats and displays events

### Configuration

Workflow prompts are in `config/workflows.yaml` using Go templates. Config loads via Viper with env var overrides (`BMAD_` prefix).

### Claude CLI Integration

The executor always passes `--enable-auto-mode` and `--output-format stream-json`. Each JSON line from stdout is parsed into `StreamEvent` structs, then converted to the higher-level `Event` type with convenience methods (`IsText()`, `IsToolUse()`, `IsToolResult()`).
