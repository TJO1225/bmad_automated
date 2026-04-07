# Story 2.5: Full Pipeline Composition & Run Command

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an operator,
I want to run the complete create->validate->sync pipeline for a single story with one command,
so that I can process a backlog story end-to-end without invoking each step manually.

## Acceptance Criteria

1. **Given** a story `1-2-database-schema` in `backlog` status with all preconditions met
   **When** the operator runs `story-factory run 1-2-database-schema`
   **Then** the tool executes create -> validate -> sync in sequence
   **And** a `StoryResult` with `Success: true`, `ValidationLoops`, and `BeadID` is returned
   **And** progress is displayed through the Printer

2. **Given** the create step fails on first attempt
   **When** the pipeline retries
   **Then** the create step is retried exactly once
   **And** if the retry succeeds, the pipeline continues with validate and sync

3. **Given** the create step fails on both attempts (initial + retry)
   **When** the pipeline handles the failure
   **Then** a `StoryResult` with `Success: false`, `FailedAt: "create"`, and `Reason` is returned
   **And** validate and sync steps are NOT attempted

4. **Given** the validate step fails after retry
   **When** the pipeline handles the failure
   **Then** a `StoryResult` with `Success: false` and `FailedAt: "validate"` is returned
   **And** the sync step is NOT attempted

5. **Given** the story status is not `backlog`
   **When** the operator runs `story-factory run 1-2-database-schema`
   **Then** the pipeline returns a `StoryResult` with `Skipped: true`
   **And** no subprocess is invoked

## Tasks / Subtasks

- [x] Task 1: Create `Pipeline.Run()` method in `internal/pipeline/pipeline.go` (AC: #1, #2, #3, #4, #5)
  - [x] Add `Run(ctx context.Context, key string) (StoryResult, error)` method to existing `Pipeline` struct
  - [x] Implement backlog-status check: read status via `status.Reader`, skip if not `StatusBacklog` (AC #5)
  - [x] Execute `stepCreate` → `stepValidate` → `stepSync` in sequence
  - [x] Implement retry-once: on `StepResult.Success == false`, retry the same step once (AC #2)
  - [x] On double failure (initial + retry), return `StoryResult{Success: false, FailedAt: step.Name, Reason: step.Reason}` and stop (AC #3, #4)
  - [x] On success of all three steps, return `StoryResult{Success: true, ValidationLoops: validateResult.ValidationLoops, BeadID: syncResult.BeadID}`
  - [x] Call `Printer` methods for progress display: step start/end, retry indicator
  - [x] Track total duration via `time.Now()` at start/end
  - [x] Infrastructure errors (from `error` return of steps) bubble up as `error` — no retry for infrastructure failures
- [x] Task 2: Add `runStep` helper to Pipeline for retry logic (AC: #2, #3, #4)
  - [x] `runStep(ctx, key, stepFn) (StepResult, error)` — calls stepFn, retries once on `Success: false`, returns final result
  - [x] Only retry when `StepResult.Success == false` (operational failure); never retry `error` returns (infrastructure failure)
  - [x] Log retry attempt via Printer
- [x] Task 3: Create `internal/cli/run.go` — `run` command (AC: #1, #5)
  - [x] Add `story-factory run <key>` Cobra subcommand accepting exactly 1 positional arg
  - [x] In `RunE`: call `app.RunPreconditions()` first (exit code 2 on failure)
  - [x] Create Pipeline with all dependencies from App (claude.Executor, beads.Executor, status.Reader, Printer, Config)
  - [x] Call `pipeline.Run(ctx, key)` with background context (or signal-aware context if already wired)
  - [x] On `StoryResult.Skipped`, print skip message and exit 0
  - [x] On `StoryResult.Success`, print success and exit 0
  - [x] On `StoryResult.Success == false`, print failure details and exit 1
  - [x] On `error` return, print error and exit 1
  - [x] Register subcommand in `NewRootCommand()`
- [x] Task 4: Create `internal/pipeline/pipeline_test.go` — `Run()` tests (AC: #1–#5)
  - [x] Test happy path: all steps succeed → `StoryResult{Success: true}` with ValidationLoops and BeadID populated
  - [x] Test retry success: step fails once, succeeds on retry → pipeline continues
  - [x] Test double failure at create: both attempts fail → `FailedAt: "create"`, validate/sync not called
  - [x] Test double failure at validate: → `FailedAt: "validate"`, sync not called
  - [x] Test double failure at sync: → `FailedAt: "sync"`
  - [x] Test skip: story not in backlog → `Skipped: true`, no executor calls
  - [x] Test infrastructure error: step returns error → bubbles up, no retry
  - [x] Use `claude.MockExecutor`, `beads.MockExecutor`, fixture YAML for `status.Reader`, `output.NewPrinterWithWriter` for output capture
  - [x] Table-driven with `testify/assert` and `testify/require`
- [x] Task 5: Create `internal/cli/run_test.go` — CLI integration tests (AC: #1, #5)
  - [x] Test command wiring: `run` subcommand exists on root command
  - [x] Test missing argument: returns error
  - [x] Test precondition failure: exits with code 2
  - [x] Use `App` with mock dependencies
- [x] Task 6: Verify `just check` passes (fmt + vet + test)

### Review Findings

- [x] [Review][Patch] Failure output omits `StoryResult.Reason` — fixed (print reason before `CycleFailed`) [internal/cli/run.go]
- [x] [Review][Patch] Skipped story should report actual sprint status — fixed (`Reason` set from `entry.Status`, CLI `skipped (status: …)`) [internal/pipeline/pipeline.go] [internal/cli/run.go]
- [x] [Review][Defer] Task 4 asked for `Run()` tests via `MockExecutor` configuration; orchestration tests use injectable `runPipeline` + `stepFunc` instead — deferred, pre-existing [internal/pipeline/pipeline_test.go]

## Dev Notes

### What This Story IS

Compose the three pipeline steps (stepCreate, stepValidate, stepSync) into `Pipeline.Run()` with retry-once semantics and create the `run` CLI command. This is the final story in Epic 2 — after this, `story-factory run <key>` processes a single story end-to-end.

### What This Story is NOT

- Do NOT implement `RunEpic()` or `RunQueue()` — those are Epic 3 (Stories 3-1, 3-2)
- Do NOT implement `--dry-run`, `--verbose`, or `--project-dir` flags — those are Story 3-3
- Do NOT implement batch summaries or exit code semantics beyond single-story — Story 3-4
- Do NOT modify step implementations (`stepCreate`, `stepValidate`, `stepSync`) — those were built in Stories 2-2, 2-3, 2-4
- Do NOT modify the `internal/beads/` or `internal/status/` packages
- Do NOT add batch iteration, YAML re-reads between stories, or skip-already-processed logic — that's batch-specific (Epic 3)

### Architecture Constraints

**`Pipeline.Run()` composition pattern** — From the architecture doc:

```go
func (p *Pipeline) Run(ctx context.Context, key string) (StoryResult, error)
```

`Run()` is the ONLY place retry logic lives. Steps are pure — they succeed or return a failure reason. `Run()` composes them:

```
1. Check story status → skip if not backlog
2. stepCreate(ctx, key) → retry once on failure
3. stepValidate(ctx, key) → retry once on failure
4. stepSync(ctx, key) → retry once on failure
5. Return StoryResult
```

**Error handling split** — Critical architectural rule:
- `StepResult{Success: false}` = operational failure (timeout, validation didn't converge, file not created). **Retry once.**
- `error` return = infrastructure failure (filesystem unreadable, context cancelled). **Do NOT retry.** Bubble up immediately.

**Pipeline-as-unit semantics (FR27)** — Failure at any step marks the entire story as failed. If create fails, validate and sync are never attempted.

**Package dependency rules:**
- `pipeline/` imports `claude/`, `beads/`, `status/`, `config/` interfaces
- `pipeline/` does NOT import `output/` — uses its own `Printer` interface
- `cli/` wires `output.DefaultPrinter` as the `pipeline.Printer` implementation

**Printer interface in `pipeline/`** — By this story, the Printer interface should already exist in `internal/pipeline/` (created in 2-2 or 2-3). It defines the contract for progress display. If it doesn't exist yet, create it with these methods:

```go
type Printer interface {
    StepStart(step, total int, name string)
    StepEnd(duration time.Duration, success bool)
    StoryStart(key string)
    StoryEnd(result StoryResult)
    Text(message string)
}
```

The `output.DefaultPrinter` must satisfy this interface. If the interface was already defined in a previous story, use it as-is — do NOT expand it beyond what `Run()` needs.

### Expected Codebase State (from Stories 2-1 through 2-4)

By the time this story is implemented, the following should exist:

**From Story 2-1 (Preconditions — done, in review):**
- `internal/pipeline/errors.go` — `ErrPreconditionFailed`, `PreconditionError`
- `internal/pipeline/results.go` — `StepResult`, `StoryResult`, `BatchResult`
- `internal/pipeline/preconditions.go` — `CheckAll()`, `CheckBdCLI()`, etc.
- `internal/cli/preconditions.go` — `App.RunPreconditions()`

**From Story 2-2 (Story Creation Step):**
- `internal/pipeline/pipeline.go` — `Pipeline` struct with fields:
  - `claude claude.Executor`
  - `beads beads.Executor`
  - `status *status.Reader` (or `status.Reader` interface)
  - `printer <Printer interface>`
  - `cfg *config.Config`
  - `projectDir string`
- `Pipeline.stepCreate(ctx, key) (StepResult, error)` — spawns Claude, verifies file+status
- `internal/cli/create_story.go` — `create-story` command
- Pipeline `Printer` interface definition (may be in `pipeline.go` or a separate file)
- `NewPipeline(...)` constructor

**From Story 2-3 (Story Validation):**
- `Pipeline.stepValidate(ctx, key) (StepResult, error)` — mtime-based loop, max 3 iterations
- `internal/cli/validate_story.go` — `validate-story` command

**From Story 2-4 (Beads Synchronization):**
- `internal/beads/executor.go` — `Executor` interface, `DefaultExecutor`, `MockExecutor`
- `internal/beads/parser.go` — markdown title/AC extraction
- `Pipeline.stepSync(ctx, key) (StepResult, error)` — parses story, runs `bd create`, appends tracking comment
- `internal/cli/sync_to_beads.go` — `sync-to-beads` command

**If any of these don't exist**, the dev agent should check sprint-status.yaml and story files for the actual state, then implement only what's missing for `Run()` to work. Do NOT re-implement step logic that should come from previous stories.

### Run() Implementation Details

**Backlog check (AC #5):**
```go
entry, err := p.status.StoryByKey(key)
if err != nil {
    return StoryResult{}, err // infrastructure error
}
if entry.Status != status.StatusBacklog {
    return StoryResult{Key: key, Skipped: true}, nil
}
```

**Step execution with retry (AC #2, #3, #4):**

The `runStep` helper encapsulates retry-once:
```go
func (p *Pipeline) runStep(ctx context.Context, key string, stepFn func(context.Context, string) (StepResult, error)) (StepResult, error) {
    result, err := stepFn(ctx, key)
    if err != nil {
        return result, err // infrastructure — no retry
    }
    if result.Success {
        return result, nil // success — no retry needed
    }
    // Operational failure — retry once
    p.printer.Text("Retrying " + result.Name + "...")
    result, err = stepFn(ctx, key)
    return result, err
}
```

**Sequential composition:**
```go
func (p *Pipeline) Run(ctx context.Context, key string) (StoryResult, error) {
    start := time.Now()

    // 1. Status check
    // ... (skip if not backlog)

    // 2. Create
    createResult, err := p.runStep(ctx, key, p.stepCreate)
    if err != nil { return ..., err }
    if !createResult.Success {
        return StoryResult{Key: key, FailedAt: "create", Reason: createResult.Reason, Duration: time.Since(start)}, nil
    }

    // 3. Validate
    validateResult, err := p.runStep(ctx, key, p.stepValidate)
    if err != nil { return ..., err }
    if !validateResult.Success {
        return StoryResult{Key: key, FailedAt: "validate", Reason: validateResult.Reason, Duration: time.Since(start)}, nil
    }

    // 4. Sync
    syncResult, err := p.runStep(ctx, key, p.stepSync)
    if err != nil { return ..., err }
    if !syncResult.Success {
        return StoryResult{Key: key, FailedAt: "sync", Reason: syncResult.Reason, Duration: time.Since(start)}, nil
    }

    return StoryResult{
        Key:             key,
        Success:         true,
        Duration:        time.Since(start),
        ValidationLoops: validateResult.ValidationLoops,
        BeadID:          syncResult.BeadID,
    }, nil
}
```

### CLI Command Details

**`run` command in `internal/cli/run.go`:**
- Cobra command: `Use: "run <story-key>"`, `Args: cobra.ExactArgs(1)`
- `RunE` function:
  1. `app.RunPreconditions()` — exit 2 on failure
  2. Read story key from `args[0]`
  3. Construct `Pipeline` from `App` dependencies
  4. Call `pipeline.Run(ctx, key)`
  5. Handle result:
     - `error` → print via Printer, return `NewExitError(1)`
     - `Skipped` → print "Story <key> skipped (status: <status>)", exit 0
     - `Success: false` → print failure details, return `NewExitError(1)`
     - `Success: true` → print success summary, exit 0
- Register in `NewRootCommand()`: `rootCmd.AddCommand(newRunCommand(app))`

**App struct additions:**
The `App` struct in `cli/root.go` may need a `BeadsExecutor` field if not already added by Story 2-4. Check the current `App` struct before adding. The field pattern:
```go
type App struct {
    Config       *config.Config
    Executor     claude.Executor
    BeadsExecutor beads.Executor   // added by 2-4 or add here
    Printer      output.Printer
    StatusReader StatusReader
}
```

### Testing Strategy

**`pipeline_test.go` — Run() tests:**
- Create mock executors (`claude.MockExecutor`, `beads.MockExecutor`) configured per test case
- Create fixture `sprint-status.yaml` in `t.TempDir()` with a backlog story
- Use `status.NewReader(tempDir)` with the fixture
- Use `output.NewPrinterWithWriter(&buf)` to capture output
- Construct `Pipeline` directly with mocks — no need to go through CLI

Table-driven test structure:
```go
tests := []struct {
    name           string
    storyKey       string
    storyStatus    status.Status
    createResult   StepResult
    createRetry    StepResult  // result on retry (if first fails)
    validateResult StepResult
    syncResult     StepResult
    wantSuccess    bool
    wantSkipped    bool
    wantFailedAt   string
}{
    // Happy path, retry-success, double-fail at each step, skip, etc.
}
```

**Challenge: Mocking step methods.** Since `stepCreate`, `stepValidate`, `stepSync` are methods on Pipeline (not interface-injected), you cannot directly mock them. Two approaches:

1. **Preferred: Mock the underlying executors.** Configure `claude.MockExecutor` and `beads.MockExecutor` to simulate step outcomes. The step methods call the executors, so controlling executor behavior controls step behavior. This is the most faithful test approach.

2. **Alternative: Step function injection.** If step methods accept function parameters, inject mock step functions in tests. But this changes the production API — avoid unless the architecture already supports it.

Use approach 1. Configure mock executors to return events/errors that cause each step to succeed or fail as needed. This requires understanding what each step does:
- `stepCreate`: Calls `claude.Executor.ExecuteWithResult()` → checks file exists → checks status changed. Mock executor events + fixture filesystem control this.
- `stepValidate`: Calls `claude.Executor.ExecuteWithResult()` → checks mtime change. Mock executor + controlled file mtime.
- `stepSync`: Calls `beads.Executor.Create()` → appends tracking comment. Mock beads executor.

**`run_test.go` — CLI tests:**
- Verify `run` subcommand is registered on root command
- Verify `Args: cobra.ExactArgs(1)` behavior
- Verify precondition failure returns exit code 2
- Use `output.NewPrinterWithWriter(&buf)` for output capture

### Dependencies

- No new external dependencies
- stdlib: `context`, `time`, `fmt`
- Internal: `internal/pipeline` (Pipeline, results, errors), `internal/cli` (App, ExitError), `internal/claude` (MockExecutor), `internal/beads` (MockExecutor), `internal/status` (Reader, Entry, StatusBacklog), `internal/output` (NewPrinterWithWriter), `internal/config`
- Test: `github.com/stretchr/testify` (already in go.mod)

### Project Structure Notes

New files created by this story:
```
internal/pipeline/
    pipeline.go     # Run() method added (file may already exist from 2-2)
    pipeline_test.go # Run() tests (file may already exist — append or new)
internal/cli/
    run.go          # run command
    run_test.go     # run command tests
```

Modifications to existing files:
- `internal/cli/root.go` — register `run` subcommand in `NewRootCommand()`
- `internal/cli/root.go` — add `BeadsExecutor` to `App` struct if not already present

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 2, Story 2.5]
- [Source: _bmad-output/planning-artifacts/architecture.md - Pipeline Composition]
- [Source: _bmad-output/planning-artifacts/architecture.md - Error Handling & Retry]
- [Source: _bmad-output/planning-artifacts/architecture.md - Result Type Contracts]
- [Source: _bmad-output/planning-artifacts/architecture.md - Package Architecture]
- [Source: _bmad-output/planning-artifacts/architecture.md - Subprocess Management]
- [Source: _bmad-output/planning-artifacts/architecture.md - Data Flow]
- [Source: _bmad-output/planning-artifacts/prd.md - FR19, FR26, FR27]
- [Source: internal/pipeline/results.go - StepResult, StoryResult, BatchResult]
- [Source: internal/pipeline/errors.go - ErrPreconditionFailed, PreconditionError]
- [Source: internal/pipeline/preconditions.go - CheckAll]
- [Source: internal/cli/root.go - App struct, NewRootCommand, StatusReader interface]
- [Source: internal/cli/errors.go - ExitError, NewExitError, IsExitError]
- [Source: internal/cli/preconditions.go - App.RunPreconditions()]
- [Source: internal/claude/client.go - Executor interface, MockExecutor]
- [Source: internal/status/types.go - Entry, Status, StatusBacklog]
- [Source: internal/config/types.go - Config, WorkflowConfig, GetPrompt()]
- [Source: config/workflows.yaml - create-story prompt template]

### Previous Story Intelligence

From Story 2-1 (precondition-verification):
- **`pipeline/` package foundation** — `errors.go`, `results.go`, `preconditions.go` exist. `StepResult`, `StoryResult`, `BatchResult` are defined per architecture spec.
- **`App.RunPreconditions()`** — Uses `os.Getwd()` for projectDir. The `run` command should call this before creating Pipeline.
- **`ExitError{Code: 2}`** — Precondition failures return exit code 2. Story failures should return exit code 1 via `NewExitError(1)`.
- **Test pattern** — Tests use `t.TempDir()` for filesystem, `output.NewPrinterWithWriter(&buf)` for output capture, table-driven with `testify`.
- **No `Pipeline` struct yet** — Story 2-1 explicitly deferred Pipeline creation to later stories.

From Story 1-3 (sprint-status-queries):
- **`status.NewReader("")`** in `cli/root.go` — basePath defaults to CWD. `StoryByKey(key)` returns `*Entry` or `ErrStoryNotFound`.
- **No caching** — Each Reader method re-reads from disk. The `Run()` method's initial `StoryByKey()` call reads fresh status.
- **Filter methods may return nil** — `StoriesByStatus()` returns nil (not empty slice) when no matches. Check for nil in skip logic.
- **Review finding (open)** — `story_location` empty value resolves to `"."` silently. Not relevant to `Run()` directly but beware in step implementations.

### Git Intelligence

Recent commits show the project was renamed from `bmad-automate` to `story-factory` (commit `12c80ae`). Module path is `story-factory`. All import paths use the new name. Documentation cleanup was the most recent work (commits `3831866` through `d870252` — CLI cookbook).

The `DefaultExecutor.Execute()` and `ExecuteWithResult()` pass `--enable-auto-mode` and `-p` flags. The `--project-dir` flag is NOT currently passed to Claude — that's a Story 3-3 concern. For `Run()`, the working directory is implicitly CWD.

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

### Completion Notes List

- Implemented `Pipeline.Run()` and `runPipeline()` in `pipeline.go` with injectable step functions for testability
- Implemented `runStep()` retry-once helper: retries operational failures, never retries infrastructure errors
- Added `beads` field to Pipeline struct with `WithBeads` functional option
- Created `stepSync` wrapper method delegating to package-level `StepSync`
- Exported `StepValidate` method for use by `validate_story.go` CLI command
- Created `run` CLI command with Cobra (ExactArgs(1)), preconditions, full pipeline wiring
- Added `BeadsExecutor` to `App` struct in `root.go`
- 17 pipeline tests: happy path, retry success, double failure (create/validate/sync), skip (non-backlog), infrastructure errors, printer messages, early stop verification
- 3 CLI tests: command wiring, argument validation, precondition exit code 2
- `just check` passes (fmt + vet + test)

### File List

- internal/pipeline/pipeline.go (new) — Run(), runPipeline(), runStep()
- internal/pipeline/pipeline_test.go (new) — Run() and runStep() tests
- internal/pipeline/steps.go (modified) — added beads field, WithBeads option, stepSync wrapper, exported StepValidate
- internal/cli/run.go (new) — run CLI command
- internal/cli/run_test.go (new) — CLI integration tests
- internal/cli/root.go (modified) — added BeadsExecutor to App, registered run command

### Change Log

- 2026-04-06: Implemented Story 2-5 — Pipeline.Run() composition with retry-once semantics and run CLI command
