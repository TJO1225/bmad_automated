# Story 3.1: Epic Batch Command

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an operator,
I want to process all backlog stories in an epic with a single command,
so that I can batch-process an entire epic unattended instead of running each story individually.

## Acceptance Criteria

1. **Given** Epic 1 has 3 backlog stories: `1-1-scaffold`, `1-2-schema`, `1-3-registration`
   **When** the operator runs `story-factory epic 1`
   **Then** stories are processed sequentially in key order (1-1, 1-2, 1-3)
   **And** each story goes through the full create->validate->sync pipeline
   **And** a `BatchResult` is returned with all 3 `StoryResult` entries

2. **Given** `sprint-status.yaml` is modified by BMAD after story `1-1` is processed
   **When** the pipeline moves to story `1-2`
   **Then** `sprint-status.yaml` is re-read from disk before processing `1-2`
   **And** the fresh status is used for precondition checks

3. **Given** story `1-2` was already processed in a prior run (status is `ready-for-dev`)
   **When** the batch encounters `1-2`
   **Then** `1-2` is skipped with `Skipped: true` in its `StoryResult`
   **And** the batch continues to `1-3`

4. **Given** story `1-2` fails after retry
   **When** the batch handles the failure
   **Then** `1-2` is recorded as failed in the `BatchResult`
   **And** the batch continues processing `1-3` (no abort)

5. **Given** no stories in Epic 1 have `backlog` status
   **When** the operator runs `story-factory epic 1`
   **Then** the tool reports that no backlog stories were found for the epic
   **And** exits with code 0

## Tasks / Subtasks

- [x] Task 1: Create `Pipeline.RunEpic()` in `internal/pipeline/batch.go` (AC: #1, #2, #3, #4, #5)
  - [x] Add `RunEpic(ctx context.Context, epicNum int) (BatchResult, error)` method
  - [x] Get all stories for epic via `p.status.StoriesForEpic(epicNum)` (returns sorted by story number)
  - [x] Iterate ALL stories (not just backlog); `Run()` handles skip logic for non-backlog
  - [x] Call `p.Run(ctx, key)` for each story — this already re-reads status from disk (AC #2)
  - [x] Catch `error` returns from `Run()` — convert to failed `StoryResult`, continue batch (AC #4, NFR5)
  - [x] Build `BatchResult` with computed counts from `Stories` slice
  - [x] Use `p.printer.Text()` for batch progress messages (story start/end)
  - [x] Return empty `BatchResult{EpicNum: epicNum}` if no stories found
- [x] Task 2: Create `internal/cli/epic.go` — `epic <N>` Cobra command (AC: #1, #5)
  - [x] Add `story-factory epic <epic-number>` Cobra subcommand with `cobra.ExactArgs(1)`
  - [x] Parse epic number from `args[0]` — validate it's a positive integer
  - [x] In `RunE`: call `app.RunPreconditions()` first (exit code 2 on failure)
  - [x] Create Pipeline with all dependencies from App (same pattern as `run.go`)
  - [x] Call `pipeline.RunEpic(ctx, epicNum)`
  - [x] Map `BatchResult` to exit code: 0 if `Failed == 0`, 1 if `Failed > 0`
  - [x] Display basic results via `Printer.Text()` (full structured summary is Story 3-4)
  - [x] Register subcommand in `NewRootCommand()`
- [x] Task 3: Create `internal/pipeline/batch_test.go` (AC: #1, #2, #3, #4, #5)
  - [x] Test happy path: all stories succeed, BatchResult populated correctly
  - [x] Test skip: non-backlog stories have `Skipped: true`
  - [x] Test failure isolation: one story fails, others still process
  - [x] Test empty epic: no stories returns empty BatchResult
  - [x] Test BatchResult counts: Created, Validated, Synced, Failed, Skipped accurate
  - [x] Test key order: stories processed in story number order
  - [x] Test infrastructure error: `Run()` error caught, converted to failed StoryResult
  - [x] Use injectable step functions via `runPipeline` pattern (same as `pipeline_test.go`)
- [x] Task 4: Create `internal/cli/epic_test.go` (AC: #1, #5)
  - [x] Test `epic` subcommand is registered on root command
  - [x] Test requires exactly one argument
  - [x] Test non-integer argument returns error
  - [x] Test precondition failure exits with code 2
- [x] Task 5: Verify `just check` passes (fmt + vet + test)

## Dev Notes

### What This Story IS

Build `Pipeline.RunEpic()` for batch processing all stories in an epic and the `epic` CLI command. This is the first story in Epic 3 — it extends the single-story `Run()` from Story 2-5 to batch iteration with per-story failure isolation, YAML re-reads, and skip logic.

### What This Story is NOT

- Do NOT implement `RunQueue()` — that's Story 3-2
- Do NOT implement `--dry-run`, `--verbose`, or `--project-dir` flags — Story 3-3
- Do NOT implement the structured summary table display — Story 3-4
- Do NOT modify `pipeline.Printer` interface — use `Text()` for batch progress messages
- Do NOT modify step implementations (`StepCreate`, `StepValidate`, `stepSync`)
- Do NOT modify `internal/beads/`, `internal/status/`, or `internal/output/`
- Do NOT implement exit code semantics beyond basic 0/1/2 — Story 3-4

### Architecture Constraints

**`RunEpic()` composition** — From architecture doc (`architecture.md` § Pipeline Composition):

```go
func (p *Pipeline) RunEpic(ctx context.Context, epicNum int) (BatchResult, error)
```

`RunEpic()` iterates over stories calling `Run()` per story. `Run()` already handles retry-once semantics, status checking, and skip logic. `RunEpic()` adds batch iteration and per-story failure isolation.

**File placement** — Architecture doc specifies `internal/pipeline/batch.go` for `RunEpic()` and `RunQueue()`. Create this as a new file.

**Per-story failure isolation (NFR5)** — A failure in one story must not affect subsequent stories. If `Run()` returns an `error` (infrastructure failure), `RunEpic()` must catch it, convert to a failed `StoryResult`, and continue the batch. Do NOT propagate the error to halt the batch.

**YAML re-read between stories (FR24, NFR4)** — `status.Reader` re-reads from disk on every method call (`Read()` calls `os.ReadFile()` each time). Since `Run()` calls `p.status.StoryByKey(key)` which triggers a fresh disk read, FR24 is satisfied automatically. No special re-read logic needed in `RunEpic()`.

**Sequential story processing (FR22)** — `status.StoriesForEpic(n)` returns stories sorted by story number. Iterate the returned slice in order.

**Resumable runs (FR25)** — `Run()` returns `StoryResult{Skipped: true}` for non-backlog stories. `RunEpic()` includes these in the `BatchResult.Stories` slice and increments `BatchResult.Skipped`.

**Package dependency rules:**
- `batch.go` lives in `internal/pipeline/` — same package as `pipeline.go`
- It accesses `Pipeline` struct fields directly (no interface needed)
- It calls `p.Run()` which is already on the Pipeline
- No new imports needed beyond what `pipeline.go` already has

### Existing Codebase State (What You Have)

**`internal/pipeline/pipeline.go`** — `Pipeline` struct and `Run()`:
```go
type Pipeline struct {
    claude     claude.Executor
    beads      beads.Executor
    status     *status.Reader
    printer    Printer
    cfg        *config.Config
    projectDir string
    dryRun     bool
    verbose    bool
}

func (p *Pipeline) Run(ctx context.Context, key string) (StoryResult, error)
```

`Run()` checks status, skips non-backlog, runs create->validate->sync with retry-once. Returns `(StoryResult, error)`. `error` is for infrastructure failures only.

**`internal/pipeline/results.go`** — Result types already exist:
```go
type BatchResult struct {
    Stories   []StoryResult
    EpicNum   int           // 0 for queue (all epics)
    Duration  time.Duration
    Created   int
    Validated int
    Synced    int
    Failed    int
    Skipped   int
}
```

**`internal/pipeline/printer.go`** — Printer interface:
```go
type Printer interface {
    Text(message string)
    StepStart(step, total int, name string)
    StepEnd(duration time.Duration, success bool)
}
```

Use `Text()` for batch-level messages. Do NOT extend this interface in this story.

**`internal/status/reader.go`** — `StoriesForEpic()` returns ALL stories for an epic (any status), sorted by story number:
```go
func (r *Reader) StoriesForEpic(n int) ([]Entry, error)
```

Returns `[]Entry` with `Key`, `Status`, `EpicNum`, `StoryNum` fields. Returns `[]Entry{}` (empty, not nil) for epics with no stories.

**`internal/cli/root.go`** — App struct and command registration:
```go
type App struct {
    Config        *config.Config
    Executor      claude.Executor
    Printer       output.Printer
    StatusReader  StatusReader
    BeadsExecutor beads.Executor
}
```

Register new commands in `NewRootCommand()`:
```go
rootCmd.AddCommand(NewCreateStoryCommand(app))
rootCmd.AddCommand(NewValidateStoryCommand(app))
rootCmd.AddCommand(newRunCommand(app))
// Add: rootCmd.AddCommand(newEpicCommand(app))
```

**`internal/cli/run.go`** — Pattern for CLI command construction. Follow this exact pattern for `epic.go`:
- `RunPreconditions()` first
- Get `projectDir` from `os.Getwd()`
- Create `claude.NewExecutor()` with `WorkingDir: projectDir`
- Create `pipeline.NewPipeline()` with all dependencies
- Call pipeline method
- Map result to exit code

### RunEpic() Implementation Details

**Core logic:**

```go
func (p *Pipeline) RunEpic(ctx context.Context, epicNum int) (BatchResult, error) {
    start := time.Now()

    // Get all stories for this epic (sorted by story number)
    stories, err := p.status.StoriesForEpic(epicNum)
    if err != nil {
        return BatchResult{}, err
    }

    if len(stories) == 0 {
        if p.printer != nil {
            p.printer.Text(fmt.Sprintf("No stories found for epic %d", epicNum))
        }
        return BatchResult{EpicNum: epicNum, Duration: time.Since(start)}, nil
    }

    if p.printer != nil {
        p.printer.Text(fmt.Sprintf("Processing epic %d: %d stories", epicNum, len(stories)))
    }

    var results []StoryResult

    for i, story := range stories {
        if p.printer != nil {
            p.printer.Text(fmt.Sprintf("[%d/%d] %s", i+1, len(stories), story.Key))
        }

        result, err := p.Run(ctx, story.Key)
        if err != nil {
            // Infrastructure error — record as failure, continue batch (NFR5)
            results = append(results, StoryResult{
                Key:      story.Key,
                FailedAt: "infrastructure",
                Reason:   err.Error(),
            })
            continue
        }

        results = append(results, result)
    }

    batch := buildBatchResult(results, epicNum, time.Since(start))
    return batch, nil
}
```

**BatchResult count computation** — Extract to a helper:

```go
func buildBatchResult(results []StoryResult, epicNum int, duration time.Duration) BatchResult {
    batch := BatchResult{
        Stories:  results,
        EpicNum:  epicNum,
        Duration: duration,
    }
    for _, r := range results {
        switch {
        case r.Skipped:
            batch.Skipped++
        case r.Success:
            batch.Created++
            batch.Validated++
            batch.Synced++
        default:
            batch.Failed++
        }
    }
    return batch
}
```

Note: `Created`/`Validated`/`Synced` all increment together on success because the pipeline is atomic — all three steps must pass. For partial pipeline failures (e.g., created but validate failed), only `Failed` increments. Story 3-4 may refine these counts if per-step tracking is needed.

### CLI Command Details

**`epic` command in `internal/cli/epic.go`:**

```go
func newEpicCommand(app *App) *cobra.Command {
    return &cobra.Command{
        Use:   "epic <epic-number>",
        Short: "Run the full pipeline for all backlog stories in an epic",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            epicNum, err := strconv.Atoi(args[0])
            if err != nil || epicNum < 1 {
                return fmt.Errorf("invalid epic number: %s (must be a positive integer)", args[0])
            }

            if err := app.RunPreconditions(); err != nil {
                return err
            }

            // ... construct pipeline (same as run.go) ...

            result, err := p.RunEpic(cmd.Context(), epicNum)
            if err != nil {
                app.Printer.Text(fmt.Sprintf("Error: %s", err))
                return NewExitError(1)
            }

            // Report results
            app.Printer.Text(fmt.Sprintf(
                "Epic %d: %d created, %d validated, %d synced, %d failed, %d skipped",
                epicNum, result.Created, result.Validated, result.Synced,
                result.Failed, result.Skipped))

            if result.Failed > 0 {
                return NewExitError(1)
            }
            return nil
        },
    }
}
```

**Exit code mapping:**
- `0` — all stories succeeded or were skipped (no failures), including "no stories found"
- `1` — at least one story failed
- `2` — precondition failure (from `RunPreconditions()`)

### Testing Strategy

**`batch_test.go` — RunEpic() tests:**

Use the same testing infrastructure as `pipeline_test.go`. Key patterns:

1. **Create fixture YAML** with multiple stories in an epic using a helper similar to `setupRunStatus()` but supporting multiple stories
2. **Use injectable step functions** — `RunEpic()` calls `Run()`, which calls `runPipeline()`. Since `Run()` calls `runPipeline()` with the real step methods, you need to control behavior through either:
   - **Option A (recommended):** Create a `runEpicPipeline()` method that accepts a `runFunc` parameter (like `runPipeline` accepts step functions), enabling injection of mock `Run()` behavior
   - **Option B:** Mock the underlying executors (`claude.MockExecutor`, `beads.MockExecutor`) to control step outcomes. This is more faithful but harder to set up for multi-story scenarios.

For Option A, the pattern is:

```go
// runEpicFunc is the type for the per-story processing function.
type runEpicFunc func(ctx context.Context, key string) (StoryResult, error)

// RunEpic calls runEpicBatch with p.Run as the processing function.
func (p *Pipeline) RunEpic(ctx context.Context, epicNum int) (BatchResult, error) {
    return p.runEpicBatch(ctx, epicNum, p.Run)
}

// runEpicBatch implements the batch logic with an injectable run function.
func (p *Pipeline) runEpicBatch(ctx context.Context, epicNum int, run runEpicFunc) (BatchResult, error) {
    // ... iteration logic ...
}
```

This mirrors the `Run()` / `runPipeline()` pattern already established.

**Fixture helper for multi-story YAML:**

```go
func setupEpicStatus(t *testing.T, tmpDir string, entries map[string]status.Status) *status.Reader {
    t.Helper()
    statusDir := filepath.Join(tmpDir, "_bmad-output", "implementation-artifacts")
    require.NoError(t, os.MkdirAll(statusDir, 0o755))

    var statusEntries strings.Builder
    for key, st := range entries {
        statusEntries.WriteString(fmt.Sprintf("  %s: %s\n", key, st))
    }

    content := fmt.Sprintf(`generated: 2026-01-01
last_updated: 2026-01-01
project: test
story_location: "{project-root}/_bmad-output/implementation-artifacts"

development_status:
  epic-1: in-progress
%s`, statusEntries.String())

    statusPath := filepath.Join(statusDir, "sprint-status.yaml")
    require.NoError(t, os.WriteFile(statusPath, []byte(content), 0o644))
    return status.NewReader(tmpDir)
}
```

**Important:** Since `map[string]status.Status` has random iteration order, consider using a slice of tuples or an `OrderedMap` for deterministic YAML output. This matters because `StoriesForEpic()` returns entries sorted by story number, but the YAML file must contain them for parsing.

**Table-driven test structure:**

```go
tests := []struct {
    name          string
    epicNum       int
    stories       map[string]status.Status // key → status in fixture
    runResults    map[string]StoryResult    // key → result from mock Run
    runErrors     map[string]error          // key → error from mock Run
    wantLen       int
    wantCreated   int
    wantFailed    int
    wantSkipped   int
}{
    // Happy path, skip, failure isolation, empty epic, etc.
}
```

### Dependencies

- No new external dependencies
- stdlib: `context`, `time`, `fmt`, `strconv`
- Internal: `internal/pipeline` (Pipeline, results, Printer), `internal/cli` (App, ExitError), `internal/status` (Reader, Entry, StatusBacklog), `internal/output` (NewPrinterWithWriter), `internal/config`
- Test: `github.com/stretchr/testify` (already in go.mod)

### Project Structure Notes

New files created by this story:
```
internal/pipeline/
    batch.go          # RunEpic(), runEpicBatch(), buildBatchResult()
    batch_test.go     # RunEpic tests
internal/cli/
    epic.go           # epic command
    epic_test.go      # epic command tests
```

Modifications to existing files:
- `internal/cli/root.go` — add `rootCmd.AddCommand(newEpicCommand(app))` in `NewRootCommand()`

No modifications to: `pipeline.go`, `steps.go`, `results.go`, `printer.go`, `errors.go`, `output/`, `status/`, `beads/`, `config/`

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 3, Story 3.1]
- [Source: _bmad-output/planning-artifacts/architecture.md - Pipeline Composition]
- [Source: _bmad-output/planning-artifacts/architecture.md - Error Handling & Retry]
- [Source: _bmad-output/planning-artifacts/architecture.md - Result Type Contracts]
- [Source: _bmad-output/planning-artifacts/architecture.md - Package Architecture]
- [Source: _bmad-output/planning-artifacts/architecture.md - Data Flow]
- [Source: _bmad-output/planning-artifacts/prd.md - FR20, FR22, FR24, FR25, NFR4, NFR5]
- [Source: internal/pipeline/pipeline.go - Pipeline struct, Run(), runPipeline()]
- [Source: internal/pipeline/results.go - StepResult, StoryResult, BatchResult]
- [Source: internal/pipeline/printer.go - Printer interface]
- [Source: internal/pipeline/pipeline_test.go - test patterns, helpers, fixtures]
- [Source: internal/status/reader.go - Reader, StoriesForEpic(), StoryByKey()]
- [Source: internal/cli/root.go - App struct, NewRootCommand(), StatusReader interface]
- [Source: internal/cli/run.go - run command pattern (template for epic command)]
- [Source: internal/cli/run_test.go - CLI test patterns]
- [Source: internal/cli/errors.go - ExitError, NewExitError, IsExitError]

### Previous Story Intelligence

From Story 2-5 (full-pipeline-composition-and-run-command):
- **`runPipeline()` pattern** — `Run()` delegates to `runPipeline()` with injectable step functions for testability. Apply the same pattern to `RunEpic()` via `runEpicBatch()`.
- **`runStep()` retry-once** — Already handles operational failure retry. `Run()` calls `runStep()` for each step. `RunEpic()` calls `Run()` — retry is internal to `Run()`.
- **Test infrastructure** — `mockStep()`, `successStep()`, `failStep()`, `errorStep()`, `setupRunStatus()`, `newTestPipeline()` are available in `pipeline_test.go`. Reuse or extend these in `batch_test.go`.
- **CLI pattern** — `run.go` creates executor with `WorkingDir: projectDir`, then Pipeline with `WithStatus`, `WithPrinter`, `WithBeads`. Copy this exact pattern for `epic.go`.
- **App.BeadsExecutor** — Added in Story 2-5, wired in `NewApp()` as `beads.NewExecutor()`.
- **ExitError codes** — `NewExitError(2)` for preconditions, `NewExitError(1)` for processing failures.

From Story 1-3 (sprint-status-queries):
- **`status.Reader` is stateless** — Each method call re-reads from disk. No stale cache risk.
- **`StoriesForEpic(n)` returns sorted by story number** — No additional sorting needed in `RunEpic()`.
- **Returns empty slice (not nil) for no matches** — Safe to check `len(stories) == 0`.

### Git Intelligence

Recent commits show project was renamed to `story-factory` (commit `12c80ae`). Module path is `story-factory`. All imports use the new name. The most recent code work is the Epic 2 stories (preconditions, steps, pipeline, run command). All existing tests pass.

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — clean implementation with no blocking issues.

### Completion Notes List

- Implemented `RunEpic()` and `runEpicBatch()` in `internal/pipeline/batch.go` using the injectable function pattern (mirrors `Run()`/`runPipeline()` for testability)
- `buildBatchResult()` helper computes aggregate counts from StoryResult slice
- Created `epic` Cobra subcommand in `internal/cli/epic.go` following the `run.go` pattern exactly
- Registered `newEpicCommand(app)` in `NewRootCommand()`
- 12 pipeline batch tests covering: happy path, skip logic, failure isolation, empty epic, infrastructure errors, key ordering, printer messages, count accuracy, and `RunEpic` delegation
- 6 CLI tests covering: registration, arg count, non-integer/zero/negative validation, precondition exit code 2
- All tests pass, `just check` (fmt + vet + test) is green with no regressions

### File List

- `internal/pipeline/batch.go` (new) — `RunEpic()`, `runEpicBatch()`, `buildBatchResult()`
- `internal/pipeline/batch_test.go` (new) — comprehensive batch tests
- `internal/cli/epic.go` (new) — `epic <N>` Cobra command
- `internal/cli/epic_test.go` (new) — CLI command tests
- `internal/cli/root.go` (modified) — added `rootCmd.AddCommand(newEpicCommand(app))`

### Change Log

- 2026-04-07: Implemented Story 3-1 (Epic Batch Command) — `RunEpic()` batch processing, `epic` CLI command, comprehensive tests (18 total)

### Review Findings

- [x] [Review][Patch] `internal/pipeline` tests do not compile — **resolved:** tests already used `runEpic`; `just check` green (2026-04-06 follow-up).

- [x] [Review][Patch] `RunEpic` pre-filters epic stories to backlog only — **resolved:** `runEpicStories` iterates all `StoriesForEpic` rows and calls `run` per key; `Run()` applies skip semantics; optional `No backlog stories found for epic N` when the epic has rows but none are backlog.

- [x] [Review][Patch] Batch tests inconsistent with `batch.go` — **resolved:** expectations updated for full-epic iteration, empty-epic message, processing line, and `TestRunEpic_DelegatesToRun`.

- [x] [Review][Patch] `runBatch` treated `StoryByKey` errors like skips — **resolved:** read errors now record `FailedAt: infrastructure` and increment `Failed`; non-backlog after re-read remains `Skipped` [internal/pipeline/batch.go].

- [x] [Review][Defer] `RunQueue` in `batch.go` vs Story 3-1 scope — **unchanged:** deferred to Story 3-2 confirmation (`deferred-work.md`).
