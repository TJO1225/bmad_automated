# Story 2.3: Story Validation with Auto-Accept Loop

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an operator,
I want to validate a story with automatic suggestion acceptance up to 3 iterations,
so that story quality is ensured without manual review of each suggestion.

## Acceptance Criteria

1. **Given** a story with status `ready-for-dev` and an existing story file
   **When** the operator runs `story-factory validate-story 1-2-database-schema`
   **Then** a `claude -p` subprocess is spawned with `--enable-auto-mode`
   **And** the working directory is set to the project directory

2. **Given** the story file mtime does not change after a validation invocation
   **When** the step checks the mtime
   **Then** validation is considered clean (converged)
   **And** a `StepResult` with `Success: true` and `ValidationLoops: 1` is returned

3. **Given** the story file mtime changes after validation (suggestions were applied)
   **When** the step detects the mtime change
   **Then** validation is re-invoked automatically
   **And** this continues up to a maximum of 3 total iterations

4. **Given** the story file mtime still changes on the 3rd iteration
   **When** the maximum iteration count is reached
   **Then** a `StepResult` with `Success: false` and reason `needs-review` is returned
   **And** `ValidationLoops: 3` is recorded

5. **Given** validation converges on the 2nd iteration (mtime unchanged)
   **When** the step completes
   **Then** a `StepResult` with `Success: true` and `ValidationLoops: 2` is returned

## Tasks / Subtasks

- [x] Task 1: Add `stepValidate()` method to Pipeline struct in `internal/pipeline/steps.go` (AC: #1–#5)
  - [x] Accept `ctx context.Context` and `key string` parameters
  - [x] Resolve story file path using the story location pattern and key
  - [x] Implement mtime-based loop: stat before → invoke claude → stat after → compare
  - [x] Use same `create-story` prompt template (BMAD auto-detects validation when file exists)
  - [x] Return `StepResult{Name: "validate", Success: true, ValidationLoops: N}` on convergence
  - [x] Return `StepResult{Name: "validate", Success: false, Reason: "needs-review", ValidationLoops: 3}` on exhaustion
  - [x] Track `Duration` from start to completion
- [x] Task 2: Add constants for validation configuration in `internal/pipeline/steps.go`
  - [x] `maxValidationLoops = 3`
  - [x] `stepNameValidate = "validate"`
- [x] Task 3: Create `internal/pipeline/steps_test.go` — unit tests for stepValidate (AC: #2–#5)
  - [x] Test convergence on first loop (mtime unchanged after Claude) → Success, ValidationLoops: 1
  - [x] Test convergence on 2nd loop (mtime changes once, then stable) → Success, ValidationLoops: 2
  - [x] Test exhaustion after 3 loops (mtime always changes) → Failure, reason: "needs-review", ValidationLoops: 3
  - [x] Test Claude subprocess failure returns infrastructure error
  - [x] Test missing story file returns infrastructure error
  - [x] Test context cancellation propagates correctly
  - [x] Use table-driven tests with `testify/assert` and `testify/require`
- [x] Task 4: Create `internal/cli/validate_story.go` — CLI command (AC: #1)
  - [x] Add `validate-story` Cobra command accepting `<story-key>` argument
  - [x] Run preconditions first via `app.RunPreconditions()`
  - [x] Look up story via `app.StatusReader.StoryByKey()` — verify it exists
  - [x] Create Pipeline instance and call `StepValidate()`
  - [x] Display result via Printer and return appropriate exit code
  - [x] Register command in `NewRootCommand()` in `root.go`
- [x] Task 5: Create `internal/cli/validate_story_test.go` — CLI integration tests (AC: #1)
  - [x] Test command wiring (validate-story registered, accepts key argument)
  - [x] Test precondition failure returns exit code 2
  - [x] Test story not found returns appropriate error
  - [x] Use `output.NewPrinterWithWriter(buf)` to capture output
- [x] Task 6: Verify `just check` passes (fmt + vet + test)

### Review Findings

- [x] [Review][Patch] StepValidate passes a nil event handler — **fixed:** verbose + printer now use the same text-forwarding handler as StepCreate.
- [x] [Review][Patch] Story path hardcoded; CLI missing file check — **fixed:** `StepValidate` resolves via `status.Reader.ResolveStoryLocation`; `validate-story` checks `Stat` after resolve.
- [x] [Review][Patch] No per-loop timeout — **fixed:** each invocation uses `context.WithTimeout(..., DefaultTimeout)` with the same deadline/cancel handling pattern as StepCreate.
- [x] [Review][Patch] Flaky story-not-found test — **fixed:** `App.CheckPreconditions` hook + assert exit code 1 and message.

## Dev Notes

### What This Story IS

Implement the validation step of the pipeline — the mtime-based loop that re-invokes Claude up to 3 times until the story file stops changing. This includes:
1. The `stepValidate()` method on the `Pipeline` struct (created by story 2.2)
2. The `validate-story` CLI command
3. Tests for both layers

### What This Story is NOT

- Do NOT implement `stepCreate()` — that's story 2.2 (should already exist)
- Do NOT implement `stepSync()` — that's story 2.4
- Do NOT implement `Run()`, `RunEpic()`, `RunQueue()` — that's story 2.5
- Do NOT create the `internal/beads/` package — that's story 2.4
- Do NOT add retry logic — retry-once semantics are in `Run()` (story 2.5)
- Do NOT modify `internal/status/`, `internal/claude/`, or `internal/config/`

### Architecture Constraints

**Package dependency rules for this story:**
- `pipeline/` imports `claude/` (for `Executor` interface), `config/` (for prompt expansion), `status/` (for path resolution)
- `pipeline/` does NOT import `output/` — the `Printer` interface will be defined in `pipeline/` when needed (story 2.5), but for now output goes through the CLI layer
- `cli/` depends on `pipeline/` (for `Pipeline` and `stepValidate`), `output/` (for Printer), `status/` (for story lookup)

**Error representation:**
- `stepValidate()` returns `(StepResult, error)`
- Operational outcomes → `StepResult`: convergence (success), exhaustion after 3 loops (failure with reason `needs-review`)
- Infrastructure failures → `error`: story file missing, Claude failed to start, context cancelled, filesystem error

### Prerequisite: Story 2.2 (Story Creation Step)

Story 2.2 creates the `Pipeline` struct and `stepCreate()`. By the time this story is implemented, the following should exist:

**`internal/pipeline/pipeline.go`** — Pipeline struct:
```go
type Pipeline struct {
    claude  claude.Executor
    printer output.Printer  // or pipeline.Printer interface
    cfg     *config.Config
    dryRun  bool
    verbose bool
    projectDir string
}

func NewPipeline(claude claude.Executor, printer output.Printer, cfg *config.Config, projectDir string) *Pipeline
```

**`internal/pipeline/steps.go`** — should already have `stepCreate()` and possibly constants like `stepNameCreate`.

If the Pipeline struct doesn't exist yet (story 2.2 not completed), create it with the minimum fields needed for `stepValidate()`:
- `claude claude.Executor` — to invoke Claude subprocess
- `cfg *config.Config` — for prompt template expansion
- `projectDir string` — for resolving story file paths

### Validation Loop — Detailed Implementation

**Core algorithm:**
```
for loop = 1 to 3:
    1. os.Stat(storyFilePath) → record ModTime
    2. Build prompt: cfg.GetPrompt("create-story", key)  // same prompt as create
    3. claude.ExecuteWithResult(ctx, prompt, eventHandler)
    4. os.Stat(storyFilePath) → record new ModTime
    5. if newModTime == oldModTime → CONVERGED → return success with loop count
    6. (else) → suggestions applied, continue loop
return failure with reason "needs-review", loops: 3
```

**Why the same prompt as create-story:** BMAD auto-detects validation mode when the story file already exists at the expected path. The `/bmad:bmm:workflows:create-story` skill checks for the story file and switches to validation/review mode. No separate validation prompt is needed.

**Prompt template** (from `config/workflows.yaml`):
```
/bmad:bmm:workflows:create-story - Create story: {{.StoryKey}}. Do not ask questions.
```
Expanded via `cfg.GetPrompt("create-story", key)`.

**Mtime detection rationale (from architecture):**
> Validation loop uses mtime-based detection: compare story file modification timestamp before/after Claude invocation to determine if suggestions were applied

This avoids fragile text parsing of Claude's natural language output. If the file changed, BMAD applied suggestions. If unchanged, validation is clean.

**Event handling during validation:** During each Claude invocation, forward events to the Printer for progress display. Use the same event handler pattern as `stepCreate()` — if verbose mode is on, show streaming text; otherwise, just consume events silently.

### Story File Path Resolution

The story file path is derived from the sprint-status.yaml `story_location` template:
```
{project-root}/_bmad-output/implementation-artifacts/
```

For a key like `1-2-database-schema`, the expected path is:
```
<projectDir>/_bmad-output/implementation-artifacts/1-2-database-schema.md
```

Use `status.Reader.ResolveStoryLocation(projectDir)` to get the base directory, then append `<key>.md`. OR construct the path directly:
```go
storyPath := filepath.Join(p.projectDir, "_bmad-output", "implementation-artifacts", key+".md")
```

Prefer using `status.DefaultStatusPath` parent directory or the reader's resolution method to stay consistent with story 2.2's approach.

### Testing Strategy

**Pipeline tests (`internal/pipeline/steps_test.go`):**

The key challenge: simulating mtime changes. `claude.MockExecutor` doesn't modify files — it just returns events. To test the mtime loop:

**Approach: Custom mock executor that modifies the file during execution.**

```go
// testValidationExecutor wraps MockExecutor and optionally modifies the story
// file during execution to simulate BMAD applying suggestions.
type testValidationExecutor struct {
    filePath     string
    modifyOnCall []bool // per-call: true = touch file (simulate suggestions), false = leave unchanged
    callCount    int
    exitCodes    []int  // per-call exit codes
}

func (m *testValidationExecutor) ExecuteWithResult(ctx context.Context, prompt string, handler EventHandler) (int, error) {
    idx := m.callCount
    m.callCount++
    if idx < len(m.modifyOnCall) && m.modifyOnCall[idx] {
        // Simulate BMAD modifying the story file (applying suggestions)
        os.WriteFile(m.filePath, []byte(fmt.Sprintf("modified-%d", idx)), 0644)
    }
    exitCode := 0
    if idx < len(m.exitCodes) {
        exitCode = m.exitCodes[idx]
    }
    return exitCode, nil
}

func (m *testValidationExecutor) Execute(ctx context.Context, prompt string) (<-chan Event, error) {
    // Not used by stepValidate, but required by interface
    ch := make(chan Event)
    close(ch)
    return ch, nil
}
```

**Test cases (table-driven):**

| Test Case | modifyOnCall | Expected Result |
|-----------|-------------|-----------------|
| Clean on first pass | `[false]` | Success, ValidationLoops: 1 |
| Converge on 2nd pass | `[true, false]` | Success, ValidationLoops: 2 |
| Converge on 3rd pass | `[true, true, false]` | Success, ValidationLoops: 3 |
| Exhaustion after 3 | `[true, true, true]` | Failure, reason: "needs-review", ValidationLoops: 3 |
| Claude fails | N/A (return error) | Infrastructure error |
| Story file missing | N/A | Infrastructure error from os.Stat |

**CLI tests (`internal/cli/validate_story_test.go`):**
- Test command registration: verify `validate-story` exists in root command's subcommands
- Test with mock dependencies: construct `App` directly with mock executor and status reader
- Use `output.NewPrinterWithWriter(&buf)` to capture output

### `validate-story` CLI Command Details

```
Usage: story-factory validate-story <story-key>
```

**Behavior:**
1. Parse `<story-key>` argument (exactly 1 required)
2. Run `app.RunPreconditions()` — exit code 2 on failure
3. Look up story via `app.StatusReader.StoryByKey(key)` — verify it exists and status is `ready-for-dev`
4. Resolve story file path — verify file exists on disk
5. Create Pipeline and call `stepValidate(ctx, key)`
6. Display result: success message with loop count, or failure message with "needs-review" suggestion
7. Return exit code 0 on success, 1 on validation exhaustion

### Existing Code Patterns to Follow

**From `cli/preconditions.go`:** Pattern for extracting project directory:
```go
projectDir, err := os.Getwd()
```

**From `claude/client.go`:** The executor always adds `--enable-auto-mode` and `--output-format stream-json`. The step just needs to provide the prompt.

**From `config/config.go`:** Prompt expansion:
```go
prompt, err := p.cfg.GetPrompt("create-story", key)
```

**From `pipeline/results.go`:** StepResult fields:
- `Name string` — use `"validate"`
- `Success bool` — true if converged, false if exhausted
- `Reason string` — empty on success, `"needs-review"` on exhaustion
- `Duration time.Duration` — total time across all loops
- `ValidationLoops int` — number of loops executed (1–3)

### Dependencies

- No new external dependencies
- stdlib: `os` (Stat), `path/filepath`, `context`, `time`, `fmt`
- Internal: `internal/claude` (Executor), `internal/config` (GetPrompt), `internal/status` (path constants), `internal/pipeline` (StepResult, existing types)
- Test: `github.com/stretchr/testify` (already in go.mod)

### Project Structure Notes

New files created by this story:
```
internal/pipeline/
    steps.go               # stepValidate() method on Pipeline (ADD to existing file from 2.2, or create if needed)
    steps_test.go          # Validation step tests
internal/cli/
    validate_story.go      # validate-story Cobra command
    validate_story_test.go # CLI integration tests
    root.go                # MODIFY: register validate-story subcommand
```

If story 2.2 has not yet created `steps.go` or the `Pipeline` struct in `pipeline.go`, create them with the minimum needed for validation.

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 2, Story 2.3]
- [Source: _bmad-output/planning-artifacts/architecture.md - Pipeline Composition]
- [Source: _bmad-output/planning-artifacts/architecture.md - Validation Loop]
- [Source: _bmad-output/planning-artifacts/architecture.md - Error Handling & Retry]
- [Source: _bmad-output/planning-artifacts/architecture.md - Subprocess Invocation Rules]
- [Source: _bmad-output/planning-artifacts/architecture.md - Test Patterns]
- [Source: _bmad-output/planning-artifacts/prd.md - FR10, FR11, FR12, FR13]
- [Source: config/workflows.yaml - create-story prompt template]
- [Source: internal/claude/client.go - Executor interface, MockExecutor]
- [Source: internal/pipeline/results.go - StepResult definition]
- [Source: internal/config/config.go - GetPrompt method]

### Previous Story Intelligence

From Story 2.1 (precondition-verification):
- **`internal/pipeline/` package already exists** with `errors.go`, `results.go`, and `preconditions.go`. Do not recreate — add to it.
- **`StepResult`** is already defined in `results.go` with the exact fields needed. Use it directly.
- **`ErrPreconditionFailed`** sentinel and `PreconditionError` struct exist. Follow the same error wrapping pattern if adding new sentinel errors.
- **`App.RunPreconditions()`** exists in `cli/preconditions.go`. Call it first in the validate-story command.
- **Test patterns**: t.TempDir() for filesystem tests, `output.NewPrinterWithWriter(&buf)` for output capture, table-driven tests with testify.
- **All tests pass with `just check`** — maintain this standard.

From Story 1.3 (sprint-status-queries):
- **`status.DefaultStatusPath`** = `"_bmad-output/implementation-artifacts/sprint-status.yaml"` — the story file directory is the parent of this path.
- **`status.Reader.ResolveStoryLocation(projectDir)`** resolves `{project-root}` in the story_location template. Use this to find story files.

### Git Intelligence

Recent commits:
- `12c80ae` — refactor: rename project from bmad-automate to story-factory. Module path is `story-factory`. All imports reflect the new name.
- The project is pure Go with `just` as build system. Run `just check` for fmt + vet + test.

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

- Filesystem mtime resolution issue: initial tests failed because file writes within the same millisecond didn't change mtime. Fixed by using `os.Chtimes()` in the test mock to explicitly advance timestamps.
- Linter interference: An automated linter kept adding code from future stories (beads package, StepSync, run command). Required repeated cleanup and adaptation to coexist with linter additions.

### Completion Notes List

- Implemented `StepValidate()` on Pipeline struct with mtime-based convergence loop (up to 3 iterations)
- Created `testValidationExecutor` custom mock that modifies story files during execution to simulate BMAD applying suggestions
- 8 unit tests covering all convergence/exhaustion scenarios, infrastructure errors, and context cancellation
- `validate-story` CLI command with precondition checks, story status verification, and appropriate exit codes
- 4 CLI integration tests covering command registration, argument validation, precondition failure, and story lookup
- All tests pass via `just check` (fmt + vet + test)
- Exported `StepValidate()` (was `stepValidate()`) to make it callable from CLI layer
- Renamed `setupSprintStatus` helper in `preconditions_test.go` to `setupMinimalSprintStatus` to avoid conflict with linter-generated test helpers

### File List

- internal/pipeline/steps.go (modified — added StepValidate method and validation constants)
- internal/pipeline/steps_test.go (new — 8 unit tests for StepValidate)
- internal/cli/validate_story.go (new — validate-story Cobra command)
- internal/cli/validate_story_test.go (new — 4 CLI integration tests)
- internal/cli/root.go (modified — registered validate-story subcommand)
- internal/pipeline/preconditions_test.go (modified — renamed helper to avoid conflict)
- _bmad-output/implementation-artifacts/sprint-status.yaml (modified — status updates)

### Change Log

- 2026-04-06: Implemented story 2-3 — StepValidate mtime-based loop, validate-story CLI command, and comprehensive tests
