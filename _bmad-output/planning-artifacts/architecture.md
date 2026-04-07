---
stepsCompleted:
  - 1
  - 2
  - 3
  - 4
  - 5
  - 6
  - 7
  - 8
status: 'complete'
completedAt: '2026-04-06'
lastStep: 8
inputDocuments:
  - _bmad-output/planning-artifacts/prd.md
  - _bmad-output/planning-artifacts/product-brief-story-factory.md
  - _bmad-output/planning-artifacts/product-brief-story-factory-distillate.md
  - docs/story-factory-implementation-spec.md
workflowType: 'architecture'
project_name: 'bmad_automated'
user_name: 'Tom'
date: '2026-04-06'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

## Project Context Analysis

### Requirements Overview

**Functional Requirements (36 total, 7 categories):**

| Category | FRs | Architectural Implication |
|----------|-----|---------------------------|
| Sprint Status Management | FR1–FR6 | New flat YAML parser package; path template resolution; entry classification (epic/story/retro) |
| Story Creation | FR7–FR9 | Claude subprocess executor with streaming JSON; post-condition verification (file exists, status changed) |
| Story Validation | FR10–FR13 | Stateful re-invocation loop (max 3 iterations); auto-acceptance logic; `needs-review` terminal state |
| Beads Synchronization | FR14–FR18 | New `bd create` integration package; markdown parsing (title, ACs); file mutation (append bead tracking comment) |
| Pipeline Orchestration | FR19–FR27 | Sequential composition of create→validate→sync; batch iteration with YAML re-reads; retry-once-then-skip; resumable via status check |
| Precondition Verification | FR28–FR31 | Pre-flight checks (PATH lookup, file existence, agent file presence); fail-fast with exit code 2 |
| Reporting & Output | FR32–FR36 | Dry-run mode (plan without execute); verbose streaming; structured summary with per-story detail; exit code semantics (0/1/2) |

**Non-Functional Requirements (10 total, 3 categories):**

| Category | NFRs | Architectural Implication |
|----------|------|---------------------------|
| Integration | NFR1–NFR4 | Must conform to Claude CLI streaming protocol and `--enable-auto-mode`; BMAD v6 YAML format strict parsing; `bd` output parsing with graceful failure; sprint-status.yaml treated as externally mutable |
| Reliability | NFR5–NFR8 | Per-story failure isolation; read-only sprint-status (corruption-safe by design); subprocess cleanup on all exit paths; idempotent resumability |
| Performance | NFR9–NFR10 | Configurable subprocess timeout (default 5 min); tool startup < 2 seconds |

**Scale & Complexity:**

- Primary domain: CLI tool / developer workflow automation
- Complexity level: Low
- Estimated architectural components: 6-7 packages (cli, status, claude, beads, pipeline, output, config)

### Technical Constraints & Dependencies

- **Language:** Go (fork constraint — single binary distribution)
- **Build system:** `just` task runner
- **Retained packages:** `internal/claude/` (subprocess + JSON streaming), `internal/config/` (Viper), `internal/output/` (terminal formatting)
- **External CLI dependencies:** `claude` (Claude Code CLI), `bd` (Gastown Beads CLI)
- **BMAD v6 contract:** Flat `development_status` YAML map — not the nested `sprints[].stories[]` format the original fork expects
- **Subprocess model:** Fresh `claude -p` per invocation — no context carryover, no shared state between steps
- **Sprint-status.yaml ownership:** BMAD writes, Story Factory reads — never the reverse

### Cross-Cutting Concerns Identified

1. **Subprocess lifecycle management** — Timeout handling, orphan prevention, cleanup on crash/interrupt for both `claude` and `bd` processes (NFR7)
2. **Resumable batch state** — Every batch command must re-read sprint-status.yaml between operations and skip already-processed stories (FR24, FR25, NFR4)
3. **Structured error reporting** — Failures must be collected per-story and surfaced in the final summary, not lost in stdout noise (FR35, NFR5)
4. **Post-condition verification** — After each pipeline step, verify the expected side effect occurred (file created, status changed, bead ID returned) before proceeding (FR8, FR9)
5. **Dry-run threading** — `--dry-run` must propagate through every layer without executing any subprocess, while still producing accurate operation plans (FR32)

## Starter Template Evaluation

### Primary Technology Domain

**CLI Tool (Go)** — brownfield fork of `robertguss/bmad_automated`. The technology stack is established and non-negotiable. This step evaluates the existing foundation rather than selecting a new starter.

### Existing Foundation Assessment

**Language & Runtime:**
- Go 1.25.5, single binary distribution
- Module: `bmad-automate` (to be renamed `story-factory`)

**Core Dependencies (go.mod):**

| Dependency | Version | Purpose | Retain? |
|-----------|---------|---------|---------|
| `spf13/cobra` | v1.10.2 | CLI framework | Yes — command structure |
| `spf13/viper` | v1.21.0 | Config loading with env var overrides | Yes — config system |
| `charmbracelet/lipgloss` | v1.1.0 | Terminal styling | Yes — output formatting |
| `stretchr/testify` | v1.11.1 | Test assertions | Yes — testing |
| `go-yaml` (indirect) | v3.0.4 | YAML parsing | Yes — sprint-status.yaml |

**Build Tooling:**
- `just` task runner with recipes for build, test, lint, coverage, fmt, vet
- `golangci-lint` for linting
- Binary output: `./bmad-automate` (to become `./story-factory`)

**Testing Infrastructure:**
- `testify` for assertions
- `claude.MockExecutor` for subprocess isolation in tests
- `output.NewPrinterWithWriter(buf)` for output capture
- Standard `go test ./...` with coverage support

### Package Disposition Plan

**Retain (keep, simplify where noted):**

| Package | Key Types | Notes |
|---------|-----------|-------|
| `internal/claude/` | `Executor` interface, `DefaultExecutor`, `MockExecutor`, `Parser`, `Event` | Core engine. Already generic "run prompt, return events." Remove any workflow-specific logic if present. |
| `internal/config/` | `Config`, `Loader`, `PromptData` | Viper-based config. Simplify to single prompt template. Remove `FullCycleConfig` and multi-workflow templates. |
| `internal/output/` | `Printer` interface, `DefaultPrinter`, `StepResult`, `StoryResult` | Terminal formatting. Already has `QueueSummary`, `StoryResult` — strong fit. May need minor additions for bead sync output. |

**Rewrite completely:**

| Package | Current | New Purpose |
|---------|---------|-------------|
| `internal/status/` | Reads flat `development_status` YAML but has `Writer` (Story Factory is read-only) | New read-only parser: `Reader` with `BacklogStories()`, `StoriesForEpic(n)`, `StoryByKey(key)`. Remove `Writer`. Entry classification (epic/story/retro). |
| `internal/cli/` | 8+ commands including `dev-story`, `code-review`, `git-commit` | 6 commands: `create-story`, `validate-story`, `sync-to-beads`, `run`, `epic`, `queue`. Remove all legacy commands. |
| `internal/router/` | Maps status → 4 workflows (create-story, dev-story, code-review, git-commit) | Simplify to pipeline step sequencing or fold into cli/pipeline logic. Only 3 transitions: create → validate → sync. |
| `internal/lifecycle/` | Generic lifecycle executor with state persistence | Replace with pipeline orchestrator for the CS→VS→sync sequence. |
| `internal/state/` | JSON state persistence for resume | Remove — resumability comes from re-reading sprint-status.yaml, not persisted state. |
| `internal/workflow/` | Generic workflow runner + QueueRunner | Simplify to pipeline runner. `QueueRunner` pattern is useful but routing logic changes. |

**Add new:**

| Package | Purpose |
|---------|---------|
| `internal/beads/` | `bd create` integration: shell out, parse bead ID from stdout, append tracking comment to story file. Story markdown parsing (title, acceptance criteria). |

### Architectural Decisions Inherited from Foundation

**Code Organization:** `internal/` package convention with dependency injection via interfaces. Commands in `cli/`, execution in domain packages. Testable via mock interfaces.

**CLI Pattern:** Cobra command tree with `App` struct for DI. `NewApp(cfg)` wires production dependencies. Commands are thin — delegate to domain packages.

**Subprocess Model:** `Executor` interface abstracts Claude CLI spawning. `ExecuteWithResult(ctx, prompt, handler)` returns exit code and streams events via callback. `MockExecutor` enables testing without real subprocesses.

**Output Model:** `Printer` interface decouples formatting from logic. All terminal output goes through `Printer` methods. Capturable via `NewPrinterWithWriter()` for test assertions.

**Config Model:** Viper loads from YAML file → env vars (`BMAD_` prefix) → defaults. Prompt templates use Go text/template with `{{.StoryKey}}` expansion.

**Note:** The first implementation story should handle the strip-and-compile phase — remove legacy commands, rename binary/module, verify `go build` succeeds.

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (Block Implementation):**
1. Package restructuring strategy → consolidate to pipeline package
2. Pipeline composition pattern → `Pipeline` struct with step methods
3. Validation loop design → encapsulated inside `stepValidate()`
4. Error handling & retry → retry at `Run()` level, steps are pure
5. Beads integration pattern → `beads.Executor` interface

**Already Decided (Inherited from Foundation):**
- Language: Go 1.25.5, CLI: Cobra, Config: Viper, Output: lipgloss, Testing: testify
- Build: `just` task runner, Subprocess: `claude.Executor` interface
- Sprint-status.yaml is read-only (BMAD writes)
- Fresh `claude -p` per invocation (no shared state)

**Deferred Decisions (Post-MVP):**
- Parallel epic processing via tmux (Phase 2)
- `--blocks` dependency chaining in Beads sync (Phase 2)
- CI/CD pipeline integration (Phase 3)

### Package Architecture

**Decision:** Consolidate `lifecycle/`, `workflow/`, `router/`, `state/` into `internal/pipeline/`. Remove `state/` entirely (resumability via sprint-status.yaml re-read, not persisted state).

**Final package structure:**

| Package | Responsibility |
|---------|---------------|
| `internal/cli/` | Cobra commands, flag parsing, `App` DI struct. Thin — delegates to `pipeline/`. |
| `internal/pipeline/` | `Pipeline` struct: create→validate→sync sequence, batch iteration (`RunEpic`, `RunQueue`), retry logic, dry-run planning, validation loop. |
| `internal/claude/` | `Executor` interface, `DefaultExecutor` (spawns `claude -p`), `MockExecutor`, streaming JSON parser, `Event` types. |
| `internal/beads/` | `Executor` interface, `DefaultExecutor` (spawns `bd create`), `MockExecutor`, story markdown parsing (title, ACs), tracking comment append. |
| `internal/status/` | Read-only `Reader`: parse flat `development_status` YAML, entry classification (epic/story/retro), filtered queries (`BacklogStories()`, `StoriesForEpic(n)`, `StoryByKey(key)`). |
| `internal/config/` | Viper-based config loading, single prompt template, `BMAD_` env var prefix. |
| `internal/output/` | `Printer` interface, lipgloss terminal formatting, `StoryResult`/`BatchResult` summary display. |

**Rationale:** 7 packages, each with a single clear responsibility. The existing `workflow.Runner` was just a thin wrapper calling `claude.Executor` and piping to `Printer` — not enough to justify its own package when `pipeline/` needs to do the same thing plus sequencing, retry, and batch logic.

### Pipeline Composition

**Decision:** `Pipeline` struct with step methods and shared context.

```go
type Pipeline struct {
    claude  claude.Executor
    beads   beads.Executor
    status  status.Reader
    printer output.Printer
    cfg     config.Config
    dryRun  bool
    verbose bool
}

func (p *Pipeline) Run(ctx context.Context, key string) StoryResult
func (p *Pipeline) RunEpic(ctx context.Context, epicNum int) BatchResult
func (p *Pipeline) RunQueue(ctx context.Context) BatchResult

// Internal steps — not exported
func (p *Pipeline) stepCreate(ctx context.Context, key string) StepResult
func (p *Pipeline) stepValidate(ctx context.Context, key string) StepResult
func (p *Pipeline) stepSync(ctx context.Context, key string) StepResult
```

**Rationale:** The pipeline has a fixed shape (always 3 steps). Interface-based chain adds abstraction without value. `Run()` composes the steps with retry; `RunEpic()`/`RunQueue()` iterate over stories calling `Run()` per story with YAML re-reads between.

### Validation Loop

**Decision:** Loop encapsulated inside `stepValidate()`, max 3 iterations.

`--enable-auto-mode` means Claude/BMAD handles suggestion acceptance automatically. The tool's job is to re-invoke and check if validation is clean. On loop exhaustion, returns `needs-review` as the failure reason. The pipeline sees validate as a single step that either passes or fails — it doesn't know about iterations.

**Rationale:** The loop is simple (invoke, check, repeat). No complex state, no suggestion parsing. A separate `Validator` type isn't justified.

### Error Handling & Retry

**Decision:** Retry at `Run()` level. Steps are pure (succeed or return failure reason).

`Run()` calls each step. On failure, retries once. On second failure, marks story as failed with `FailedAt` and `Reason`, then returns. Batch methods (`RunEpic`, `RunQueue`) collect `StoryResult` per story — a single failure never aborts the batch. Final summary reports all results.

**Exit code semantics:**
- `0` — all stories succeeded
- `1` — partial failure (at least one story failed)
- `2` — fatal precondition error (before any processing)

**Rationale:** Retry policy in one place, steps stay simple and testable. For "retry once" a generic retry wrapper is overhead.

### Beads Integration

**Decision:** `beads.Executor` interface mirroring `claude.Executor` pattern.

```go
type Executor interface {
    Create(key, title, acs string) (beadID string, err error)
    AppendTrackingComment(storyPath, beadID string) error
}
```

Includes story markdown parsing: extract title from `# Story X.Y: Title`, extract ACs from `## Acceptance Criteria` to next `##`.

**Rationale:** Matches existing codebase conventions. Enables `MockExecutor` for testing `stepSync()` without shelling out to `bd`. Small cost, high testability payoff.

### Subprocess Management

**Decision:** Context-based timeout and cleanup (inherited from `claude.Executor`).

All subprocess calls receive a `context.Context` with a configurable timeout (default 5 min per Claude invocation, NFR9). On context cancellation or timeout, subprocesses are killed. Signal handling at the CLI level cancels the root context on SIGINT/SIGTERM, propagating cleanup to all in-flight subprocesses (NFR7).

### Dry-Run Threading

**Decision:** `dryRun` flag on `Pipeline` struct, checked at step boundaries.

When `dryRun` is true, `Run()` iterates through steps but each step returns a planned action description instead of executing. `RunEpic()`/`RunQueue()` still read sprint-status.yaml to build the accurate story list, then report planned operations without invoking any subprocess.

## Implementation Patterns & Consistency Rules

### Pattern Categories Defined

**5 conflict areas identified** where AI agents could make incompatible choices across packages.

### Error Representation

- **Sentinel errors** for expected conditions: `ErrStoryNotFound`, `ErrPreconditionFailed`, `ErrStoryComplete`. Defined in the package that owns the concept.
- **Wrapped errors** with context: `fmt.Errorf("create story %s: %w", key, err)` — always include story key.
- **`StepResult` is a value, not an error.** Steps return `StepResult` with `Success bool` and `Reason string` for operational outcomes (timeout, validation didn't converge, file not created).
- **`error` return** is reserved for infrastructure failures (filesystem unreadable, YAML parse error, context cancelled). These bypass retry and bubble up as fatal.
- **Rule:** Operational outcomes → `StepResult`. Infrastructure failures → `error`.

### Result Type Contracts

All three result types live in `internal/pipeline/`:

```go
type StepResult struct {
    Name             string        // "create", "validate", "sync"
    Success          bool
    Reason           string        // empty on success
    Duration         time.Duration
    ValidationLoops  int           // validate step only
    BeadID           string        // sync step only
}

type StoryResult struct {
    Key              string
    Success          bool
    FailedAt         string        // step name, empty on success
    Reason           string
    Duration         time.Duration
    ValidationLoops  int
    BeadID           string
    Skipped          bool          // true if not in backlog (resumable)
}

type BatchResult struct {
    Stories          []StoryResult
    EpicNum          int           // 0 for queue (all epics)
    Duration         time.Duration
    Created          int
    Validated        int
    Synced           int
    Failed           int
    Skipped          int
}
```

- `output.Printer` accepts these types — does not define its own
- `BatchResult` counts computed from `Stories` slice (single source of truth)
- Steps produce `StepResult`; pipeline maps to `StoryResult`; batch methods collect into `BatchResult`

### Subprocess Invocation Rules

**Claude CLI (owned by `internal/claude/`):**
- Command: `claude -p "<prompt>" --output-format stream-json --enable-auto-mode`
- Prompt: `bmad-bmm-create-story <story-key>` (single template)
- Working directory: `--project-dir` value
- Timeout: configurable, default 5 min
- Stdout: streamed JSON events. Stderr: captured for error reporting.
- Cleanup: SIGTERM → 5s grace → SIGKILL

**Beads CLI (owned by `internal/beads/`):**
- Command: `bd create "<key>: <title>" --notes "<acs>"`
- Working directory: `--project-dir` value
- Run to completion, capture stdout. Bead ID parsed from output.
- Timeout: 30 seconds

**Mandatory rules:**
- Never invoke `claude` or `bd` outside their owning package — always via `Executor` interface
- Never hardcode CLI paths — resolve from PATH (or config override)
- Always pass `context.Context` — no subprocess without cancellable context
- Always set working directory explicitly — never rely on process CWD

### Test Patterns

**Location:** Co-located (`<file>_test.go`). Fixtures in `testdata/` per package.

**Mocks:** `Mock<InterfaceName>` in same package. Hand-written (interfaces are small). Configured per-test, no global state.

**Structure:** Table-driven for >2 cases. `testify/assert` for assertions, `testify/require` for fatal preconditions. Naming: `Test<Function>_<scenario>`.

**Coverage by layer:**

| Package | Strategy |
|---------|----------|
| `status/` | Unit tests against `testdata/` YAML fixtures. Classification, filtering, edge cases. |
| `claude/` | Unit tests for JSON parser against fixture streams. |
| `beads/` | Unit tests for markdown parsing. Mock subprocess for `Create()`. |
| `pipeline/` | Integration tests with `MockExecutor`s + real `status.Reader` on fixture YAML. Retry, validation loop, batch, dry-run. |
| `cli/` | Thin smoke tests for command wiring. |
| `config/` | Unit tests for template expansion and loading. |

### Output Conventions

**Rule: All user-facing output goes through `Printer`.** No `fmt.Println` outside `internal/output/`.

**Printer methods:** `StepStart/End`, `StoryStart/End`, `BatchSummary`, `DryRunPlan`, `PreconditionFailed`.

**Verbose mode:**
- Off: progress indicators only. Claude streaming consumed but not displayed.
- On: Claude events forwarded to `Printer.Text()` in real time. Beads stdout displayed.

**Logging:** No logging framework for MVP. Errors through `Printer`. Debug is verbose-only. If needed later, `log/slog`.

### Enforcement Guidelines

**All AI agents MUST:**
- Route all user output through `Printer` — never print directly
- Use `Executor` interfaces for all subprocess calls — never shell out directly
- Return `StepResult` for operational outcomes, `error` for infrastructure failures
- Include story key in all wrapped errors
- Write co-located tests with table-driven patterns and `testify`
- Place fixtures in `testdata/` within the relevant package

## Project Structure & Boundaries

### Complete Project Directory Structure

```
story-factory/
├── cmd/
│   └── story-factory/
│       └── main.go                    # Entry point: calls cli.Execute()
├── internal/
│   ├── cli/
│   │   ├── app.go                     # App struct, DI wiring, NewApp()
│   │   ├── root.go                    # Root Cobra command, global flags
│   │   ├── create_story.go            # create-story command
│   │   ├── validate_story.go          # validate-story command
│   │   ├── sync_to_beads.go           # sync-to-beads command
│   │   ├── run.go                     # run command (full pipeline, single story)
│   │   ├── epic.go                    # epic command (batch by epic)
│   │   ├── queue.go                   # queue command (full backlog)
│   │   ├── preconditions.go           # Pre-flight checks (bd on PATH, YAML exists, agents present)
│   │   └── cli_test.go               # Smoke tests for command wiring
│   ├── pipeline/
│   │   ├── pipeline.go                # Pipeline struct, Run(), step composition, retry
│   │   ├── batch.go                   # RunEpic(), RunQueue(), batch iteration logic
│   │   ├── steps.go                   # stepCreate(), stepValidate(), stepSync()
│   │   ├── results.go                 # StepResult, StoryResult, BatchResult types
│   │   ├── errors.go                  # Sentinel errors (ErrPreconditionFailed, etc.)
│   │   ├── pipeline_test.go           # Integration tests: retry, validation loop, batch
│   │   ├── steps_test.go             # Step-level tests with mock executors
│   │   └── batch_test.go             # Batch iteration tests with fixture YAML
│   ├── claude/
│   │   ├── executor.go                # Executor interface, DefaultExecutor
│   │   ├── mock_executor.go           # MockExecutor for tests
│   │   ├── parser.go                  # Streaming JSON parser
│   │   ├── events.go                  # Event, StreamEvent, ContentBlock types
│   │   ├── executor_test.go           # DefaultExecutor tests (subprocess mocking)
│   │   ├── parser_test.go             # JSON parser tests against fixtures
│   │   └── testdata/
│   │       ├── stream_success.jsonl   # Fixture: successful Claude stream
│   │       └── stream_error.jsonl     # Fixture: error Claude stream
│   ├── beads/
│   │   ├── executor.go                # Executor interface, DefaultExecutor
│   │   ├── mock_executor.go           # MockExecutor for tests
│   │   ├── parser.go                  # Story markdown parsing (title, ACs)
│   │   ├── executor_test.go           # bd create integration tests
│   │   ├── parser_test.go             # Markdown parsing tests
│   │   └── testdata/
│   │       ├── story_valid.md         # Fixture: well-formed story file
│   │       └── story_minimal.md       # Fixture: minimal story file
│   ├── status/
│   │   ├── reader.go                  # Reader: parse flat development_status YAML
│   │   ├── types.go                   # SprintStatus, StoryEntry, EpicEntry, Status constants
│   │   ├── reader_test.go             # Parser tests against fixture YAML
│   │   └── testdata/
│   │       ├── sprint_status.yaml     # Fixture: standard sprint status
│   │       ├── sprint_partial.yaml    # Fixture: partially processed sprint
│   │       └── sprint_empty.yaml      # Fixture: empty development_status
│   ├── config/
│   │   ├── config.go                  # Config struct, Loader, template expansion
│   │   ├── defaults.go                # Default config values
│   │   └── config_test.go             # Config loading and template tests
│   └── output/
│       ├── printer.go                 # Printer interface, DefaultPrinter
│       ├── styles.go                  # lipgloss style definitions
│       ├── printer_test.go            # Output formatting tests
│       └── testdata/                  # Expected output fixtures (if needed)
├── config/
│   └── workflows.yaml                 # Single prompt template for create-story
├── go.mod
├── go.sum
├── justfile                           # Build recipes (updated for story-factory)
├── CLAUDE.md                          # AI agent instructions (updated)
└── README.md                          # User documentation (updated)
```

### Architectural Boundaries

**Package Dependency Rules (enforced by Go import graph):**

```
cmd/story-factory/main.go
         │
         ▼
    internal/cli
         │
         ├──► internal/pipeline  (defines Printer interface + result types)
         │         │
         │         ├──► internal/claude   (Executor interface)
         │         ├──► internal/beads    (Executor interface)
         │         └──► internal/status   (Reader)
         │
         ├──► internal/output    (implements pipeline.Printer)
         │         │
         │         └──► internal/pipeline (imports result types)
         │
         └──► internal/config
```

**Boundary rules:**
- `cli/` depends on `pipeline/`, `output/`, `config/` — wires `output.DefaultPrinter` as `pipeline.Printer` implementation
- `pipeline/` defines `Printer` interface and result types. Depends on interfaces from `claude/`, `beads/`, `status/`. Never imports `output/`.
- `output/` imports `pipeline/` for result types and implements `pipeline.Printer`. No circular dependency.
- `claude/`, `beads/`, `status/` are leaf packages — depend on stdlib and external libs only, never on each other
- `config/` is a leaf package — no internal dependencies

**Integration points:**
- `claude.Executor` ↔ Claude CLI subprocess (stream-json protocol)
- `beads.Executor` ↔ Beads CLI subprocess (stdout capture)
- `status.Reader` ↔ filesystem (`sprint-status.yaml`)
- `beads.AppendTrackingComment` ↔ filesystem (story markdown files)

### Requirements to Structure Mapping

**FR Category → Package Mapping:**

| FR Category | Primary Package | Files |
|-------------|----------------|-------|
| Sprint Status Management (FR1–FR6) | `internal/status/` | `reader.go`, `types.go` |
| Story Creation (FR7–FR9) | `internal/pipeline/` | `steps.go` (stepCreate) |
| Story Validation (FR10–FR13) | `internal/pipeline/` | `steps.go` (stepValidate) |
| Beads Synchronization (FR14–FR18) | `internal/beads/` + `internal/pipeline/` | `beads/executor.go`, `beads/parser.go`, `pipeline/steps.go` (stepSync) |
| Pipeline Orchestration (FR19–FR27) | `internal/pipeline/` | `pipeline.go`, `batch.go` |
| Precondition Verification (FR28–FR31) | `internal/cli/` | `preconditions.go` |
| Reporting & Output (FR32–FR36) | `internal/output/` + `internal/cli/` | `output/printer.go`, `cli/*.go` (flag handling) |

**Cross-cutting concerns → locations:**

| Concern | Where it lives |
|---------|---------------|
| Subprocess timeout/cleanup | `internal/claude/executor.go`, `internal/beads/executor.go` (per-executor context handling) |
| Resumable batch state | `internal/pipeline/batch.go` (re-reads YAML between stories) |
| Structured error reporting | `internal/pipeline/results.go` (result types) + `internal/output/printer.go` (display) |
| Post-condition verification | `internal/pipeline/steps.go` (checked after each step) |
| Dry-run threading | `internal/pipeline/pipeline.go` (flag on struct, checked at step boundaries) |

### Data Flow

```
1. CLI parses flags → creates Pipeline with config, executors, reader, printer
2. Pipeline reads sprint-status.yaml via status.Reader → gets story list
3. For each story:
   a. stepCreate: claude.Executor runs "claude -p" → verifies file created → verifies status changed
   b. stepValidate: claude.Executor runs same command (BMAD detects validation) → loops up to 3x
   c. stepSync: beads.Executor parses story markdown → runs "bd create" → appends tracking comment
   d. Pipeline collects StepResults into StoryResult
4. Pipeline re-reads sprint-status.yaml before next story
5. Batch collects StoryResults into BatchResult
6. Printer displays BatchResult as structured summary
7. CLI maps BatchResult to exit code (0/1/2)
```

## Architecture Validation Results

### Coherence Validation ✅

**Decision Compatibility:**
All technology choices are compatible — Go 1.25.5, Cobra, Viper, lipgloss, testify, go-yaml are already working together in the current codebase. No new dependencies introduced. Interface-based DI pattern is consistent across all packages.

**Pattern Consistency:**
Error handling (StepResult for operational outcomes, error for infrastructure failures), test patterns (co-located, table-driven, testify), output conventions (all through Printer), subprocess management (context-based, Executor interface) — all internally consistent and idiomatic Go.

**Structure Alignment:**
Directory tree maps 1:1 to the 7-package architecture. Every FR category has a clear package owner. No orphaned files or ambiguous ownership.

### Requirements Coverage Validation ✅

**Functional Requirements:** All 36 FRs mapped to specific packages and files. Every FR category has a primary package owner with identified source files.

**Non-Functional Requirements:** All 10 NFRs addressed:
- Integration (NFR1–4): Executor interfaces enforce protocol compliance
- Reliability (NFR5–8): Per-story isolation in pipeline, read-only status, context-based subprocess cleanup, resumable via YAML re-read
- Performance (NFR9–10): Configurable timeout in config, precondition checks are filesystem-only (fast)

### Issues Found & Resolved

**Issue 1 — Validation result detection (Important):**
`stepValidate()` determines whether to re-invoke by checking the story file's modification timestamp before and after each Claude invocation. If mtime changed, BMAD applied suggestions — re-validate. If unchanged, validation is clean. This avoids fragile text parsing of Claude's natural language output.

**Issue 2 — Circular dependency between output/ and pipeline/ (Critical):**
Resolved by moving the `Printer` interface into `pipeline/`, alongside the result types it references. `output/` imports `pipeline/` to implement the interface. `pipeline/` never imports `output/`. `cli/` wires them together. Updated dependency graph reflected in Project Structure section.

### Architecture Completeness Checklist

**✅ Requirements Analysis**
- [x] Project context thoroughly analyzed (36 FRs, 10 NFRs categorized)
- [x] Scale and complexity assessed (low complexity, CLI tool)
- [x] Technical constraints identified (Go, brownfield fork, external CLI deps)
- [x] Cross-cutting concerns mapped (subprocess lifecycle, resumable state, error reporting, post-conditions, dry-run)

**✅ Architectural Decisions**
- [x] Package restructuring: consolidate to 7 packages
- [x] Pipeline composition: `Pipeline` struct with step methods
- [x] Validation loop: encapsulated in `stepValidate()`, mtime-based detection
- [x] Error handling: retry at `Run()` level, steps are pure
- [x] Beads integration: `Executor` interface mirroring `claude.Executor`
- [x] Subprocess management: context-based timeout and cleanup
- [x] Dry-run: flag on Pipeline struct, checked at step boundaries

**✅ Implementation Patterns**
- [x] Error representation (StepResult vs error)
- [x] Result type contracts (StepResult, StoryResult, BatchResult)
- [x] Subprocess invocation rules (flags, timeouts, working dir)
- [x] Test patterns (co-located, table-driven, mocks, fixtures)
- [x] Output conventions (all through Printer, verbose mode behavior)

**✅ Project Structure**
- [x] Complete directory structure with all files
- [x] Package dependency graph validated (no circular imports)
- [x] FR-to-package mapping complete
- [x] Cross-cutting concern locations identified
- [x] Data flow documented end-to-end

### Architecture Readiness Assessment

**Overall Status:** READY FOR IMPLEMENTATION

**Confidence Level:** High — low complexity project with well-defined integration contracts, established codebase patterns, and comprehensive decision documentation.

**Key Strengths:**
- Clean package boundaries with interface-based DI enable independent development and testing
- Brownfield approach retains proven subprocess engine and output formatting
- Pipeline-as-unit semantics with retry and resumability handle the primary failure modes
- Every FR maps to a specific file — no ambiguity for implementing agents

**Areas for Future Enhancement:**
- Parallel epic processing via tmux (Phase 2)
- `--blocks` dependency chaining in Beads sync (Phase 2)
- Structured JSON output for CI/CD consumption (Phase 2)
- `log/slog` integration if debugging needs grow beyond verbose mode

### Implementation Handoff

**AI Agent Guidelines:**
- Follow all architectural decisions exactly as documented
- Use implementation patterns consistently across all packages
- Respect package boundaries — never import across boundary rules
- Refer to this document for all architectural questions

**Implementation Sequence:**
1. **Strip and compile** — Remove legacy commands/packages, rename binary/module, verify `go build`
2. **Rewrite status parser** — New read-only `Reader` for flat YAML, with tests
3. **Build beads package** — `Executor` interface, markdown parsing, `bd create` integration
4. **Build pipeline package** — `Pipeline` struct, step methods, retry, validation loop, batch
5. **Build CLI commands** — 6 Cobra commands, precondition checks, flag handling
6. **Polish** — Update CLAUDE.md, README, justfile
