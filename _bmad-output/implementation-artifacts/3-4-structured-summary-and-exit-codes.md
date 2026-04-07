# Story 3.4: Structured Summary & Exit Codes

Status: review

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an operator,
I want a clear summary report after batch processing and meaningful exit codes,
so that I can quickly assess what landed, what failed, and script around the tool's outcomes.

## Acceptance Criteria

1. **Given** a `BatchResult` with 5 stories: 4 succeeded, 1 failed at validate
   **When** the Printer displays the summary
   **Then** a structured table shows each story with its status
   **And** successful stories show checkmark, validation loops, and bead ID
   **And** the failed story shows X, `FailedAt: "validate"`, and the failure reason
   **And** totals show: `4 created, 4 validated, 4 synced, 1 failed`

2. **Given** a queue `BatchResult` spanning multiple epics
   **When** the Printer displays the summary
   **Then** results are grouped by epic with epic headers
   **And** per-epic subtotals are shown

3. **Given** all stories in a batch succeeded
   **When** the command exits
   **Then** the exit code is `0`

4. **Given** at least one story failed but others succeeded
   **When** the command exits
   **Then** the exit code is `1`

5. **Given** a precondition check fails before any processing
   **When** the command exits
   **Then** the exit code is `2`

6. **Given** a `BatchResult` with skipped stories (already processed in prior run)
   **When** the Printer displays the summary
   **Then** skipped stories are shown with a distinct indicator (not success, not failure)
   **And** the skip count is included in totals

## Tasks / Subtasks

- [x] Task 1: Enrich `output.StoryResult` to carry full pipeline detail (AC: #1, #6)
  - [x] Add `ValidationLoops int` field
  - [x] Add `BeadID string` field
  - [x] Add `Reason string` field
  - [x] Update any existing callers/tests that construct `output.StoryResult` (may need the new fields zero-valued)

- [x] Task 2: Enhance `QueueSummary` for structured per-story detail (AC: #1, #6)
  - [x] Render each successful story row: `checkmark  story-key  loops:N  bead:ID  duration`
  - [x] Render each failed story row: `X  story-key  FailedAt: step  reason  duration`
  - [x] Render each skipped story row: `skip-icon  story-key  (skipped)` using muted style
  - [x] Display totals line: `Created: N | Validated: N | Synced: N | Failed: N | Skipped: N`
  - [x] Use lipgloss styles consistent with existing Printer (successStyle, errorStyle, mutedStyle, summaryStyle)

- [x] Task 3: Add epic-grouped summary method for queue results (AC: #2)
  - [x] Add `BatchSummary(result pipeline.BatchResult, totalDuration time.Duration)` method to `Printer` interface and `DefaultPrinter`
  - [x] OR enhance `QueueSummary` signature to accept an `epicGrouped bool` flag and a way to determine epic grouping
  - [x] Group stories by epic number (parse from story key pattern `N-M-slug` — extract first number)
  - [x] Render epic header line for each group: `Epic N` with highlight style
  - [x] Render per-epic subtotal: count of created/failed/skipped within each epic
  - [x] For single-epic runs (`epic` command, `BatchResult.EpicNum > 0`), show flat table (no grouping needed)

- [x] Task 4: Map `BatchResult` to exit codes in `epic` CLI command (AC: #3, #4, #5)
  - [x] In `internal/cli/epic.go` `RunE`: after `pipeline.RunEpic()` returns
  - [x] If `BatchResult.Failed > 0` → return `NewExitError(1)`
  - [x] If `BatchResult.Failed == 0` → return nil (exit 0)
  - [x] Precondition failure already exits 2 (from `RunPreconditions()`)

- [x] Task 5: Map `BatchResult` to exit codes in `queue` CLI command (AC: #3, #4, #5)
  - [x] In `internal/cli/queue.go` `RunE`: after `pipeline.RunQueue()` returns
  - [x] Same exit code mapping as epic command
  - [x] Precondition failure already exits 2 (from `RunPreconditions()`)

- [x] Task 6: Map `pipeline.StoryResult` to `output.StoryResult` in CLI commands (AC: #1, #2, #6)
  - [x] Create a helper function (in cli package or inline) that maps `pipeline.StoryResult` → `output.StoryResult`
  - [x] Apply mapping before passing results to Printer methods
  - [x] Alternatively, if making `output` import `pipeline` result types, remove `output.StoryResult` and use `pipeline.StoryResult` directly in the Printer interface

- [x] Task 7: Write `output/printer_test.go` tests for enhanced summary (AC: #1, #2, #6)
  - [x] Test structured table: 4 success + 1 failure → verify checkmarks, X, validation loops, bead IDs, failure reason in output
  - [x] Test epic-grouped display: stories from 2 epics → verify epic headers and per-epic subtotals
  - [x] Test all-skipped batch → verify skip indicators and skip count
  - [x] Test all-success batch → verify success header, all checkmarks, correct totals
  - [x] Test all-failed batch → verify failure header, all X marks, correct totals
  - [x] Test mixed batch with success + skipped + failed → verify all indicators and combined totals
  - [x] Use `output.NewPrinterWithWriter(&buf)` for output capture
  - [x] Assert output contains expected substrings (not exact format — allows style flexibility)

- [x] Task 8: Write CLI exit code tests (AC: #3, #4, #5)
  - [x] In `internal/cli/epic_test.go`: test exit 0 on all success, exit 1 on partial failure
  - [x] In `internal/cli/queue_test.go`: test exit 0 on all success, exit 1 on partial failure
  - [x] Precondition exit 2 already tested in existing tests — verify or add if missing
  - [x] Use mock Pipeline or mock executors to control BatchResult outcomes

- [x] Task 9: Verify `just check` passes (fmt + vet + test)

## Dev Notes

### What This Story IS

Enhance the Printer's batch summary output to show per-story details (validation loops, bead IDs, failure reasons), add epic-grouped display for queue results, and wire exit code semantics into the `epic` and `queue` CLI commands. This is the final story in Epic 3 — after this, batch operations produce comprehensive reports and meaningful exit codes for scripting.

### What This Story is NOT

- Do NOT implement `Pipeline.RunEpic()` or `Pipeline.RunQueue()` — those are Stories 3-1, 3-2 (should already exist)
- Do NOT implement `--dry-run`, `--verbose`, or `--project-dir` flags — that is Story 3-3 (should already exist)
- Do NOT modify pipeline step implementations (`StepCreate`, `StepValidate`, `stepSync`)
- Do NOT modify `pipeline.StoryResult` or `pipeline.BatchResult` types — use them as-is
- Do NOT modify `pipeline/pipeline.go` or `pipeline/batch.go` orchestration logic
- Do NOT add new pipeline steps or change retry logic

### Architecture Constraints

**Result type flow — critical pattern:**
```
Steps produce StepResult → Pipeline maps to StoryResult → Batch collects into BatchResult
                                                          → Printer displays BatchResult
                                                          → CLI maps BatchResult to exit code
```

[Source: architecture.md - Result Type Contracts]

**`BatchResult` counts are computed from `Stories` slice (single source of truth).** The batch methods (`RunEpic`, `RunQueue`) populate `BatchResult.Created`, `.Validated`, `.Synced`, `.Failed`, `.Skipped` by iterating `Stories`. The Printer should display these pre-computed counts — do NOT recount from the slice.

**Exit code semantics (from architecture):**
- `0` — all stories succeeded (including all-skipped)
- `1` — partial failure (at least one story failed)
- `2` — fatal precondition error (before any processing)

[Source: architecture.md - Exit code semantics]

**Package dependency rules:**
- `output/` does NOT import `pipeline/` currently. If you need pipeline result types in the Printer, either:
  - **(Preferred)** Enrich `output.StoryResult` with missing fields and map in the CLI layer
  - **(Alternative)** Make `output/` import `pipeline/` for result types — this is architecturally valid since `pipeline/` does NOT import `output/`, so no circular dependency
- `pipeline/` imports its own `Printer` interface (subset: `Text`, `StepStart`, `StepEnd`) — do NOT expand `pipeline.Printer`
- `cli/` owns the wiring: it calls `output.Printer` methods directly for summary display
- Summary display is NOT part of `pipeline.Printer` — it's called from CLI code after pipeline returns

[Source: architecture.md - Package Architecture]

**`output.Printer` already has these summary methods:**
```go
QueueHeader(count int, stories []string)
QueueStoryStart(index, total int, storyKey string)
QueueSummary(results []StoryResult, allKeys []string, totalDuration time.Duration)
```

These were carried from legacy code. `QueueSummary` currently shows basic completed/skipped/failed counts and per-story rows with duration. Story 3-4 needs to enrich this display with validation loops, bead IDs, and failure reasons.

**Existing `output.StoryResult` vs `pipeline.StoryResult`:**

`output.StoryResult` (current — missing fields):
```go
type StoryResult struct {
    Key      string
    Success  bool
    Duration time.Duration
    FailedAt string
    Skipped  bool
}
```

`pipeline.StoryResult` (complete):
```go
type StoryResult struct {
    Key             string
    Success         bool
    FailedAt        string
    Reason          string
    Duration        time.Duration
    ValidationLoops int
    BeadID          string
    Skipped         bool
}
```

Enrich `output.StoryResult` with `ValidationLoops`, `BeadID`, `Reason` to match.

### Expected Codebase State (from Stories 3-1 through 3-3)

By the time this story is implemented, the following should exist:

**From Story 3-1 (Epic Batch Command):**
- `internal/pipeline/batch.go` — `RunEpic(ctx, epicNum) (BatchResult, error)` method on Pipeline
- `internal/cli/epic.go` — `epic <epicNum>` Cobra command
- YAML re-read between stories, skip-already-processed, continue-on-failure semantics

**From Story 3-2 (Queue Command):**
- `internal/pipeline/batch.go` — `RunQueue(ctx) (BatchResult, error)` method on Pipeline
- `internal/cli/queue.go` — `queue` Cobra command
- Cross-epic ordering (epics in numeric order, stories in key order)
- Single `BatchResult` with all `StoryResult` entries

**From Story 3-3 (Flags):**
- `--dry-run` flag on `epic` and `queue` commands → `WithDryRun(true)` on Pipeline
- `--verbose` flag → `WithVerbose(true)` on Pipeline
- `--project-dir` flag → overrides CWD for all paths and subprocess working dirs
- `Pipeline` struct already has `dryRun` and `verbose` fields (confirmed in `steps.go`)

**If any of these don't exist**, the dev agent should check the actual codebase state and story files. Do NOT implement batch iteration or flag handling — those belong to their respective stories.

### Implementation Details

**Enhanced `QueueSummary` output format:**

For `epic` command (flat, single epic):
```
╔══════════════════════════════════════════════════════════════╗
║ BATCH COMPLETE                                               ║
╠══════════════════════════════════════════════════════════════╣
║ ──────────────────────────────────────────────────────       ║
║ ✓ 3-1-epic-batch-command         loops:1  bead:abc123  12s  ║
║ ✓ 3-2-queue-command              loops:2  bead:def456  18s  ║
║ ✗ 3-3-dry-run-flags              validate: timed out   22s  ║
║ ↷ 3-4-summary-exit-codes         (skipped)                  ║
║ ──────────────────────────────────────────────────────       ║
║ Created: 2 | Validated: 2 | Synced: 2 | Failed: 1 | Skip: 1║
║ Total: 52s                                                   ║
╚══════════════════════════════════════════════════════════════╝
```

For `queue` command (epic-grouped):
```
╔══════════════════════════════════════════════════════════════╗
║ QUEUE COMPLETE                                               ║
╠══════════════════════════════════════════════════════════════╣
║                                                              ║
║ ── Epic 1 ──────────────────────────────────────────         ║
║ ✓ 1-1-scaffold                   loops:1  bead:aaa  8s      ║
║ ✓ 1-2-schema                     loops:1  bead:bbb  12s     ║
║   Epic 1: 2 created, 0 failed                               ║
║                                                              ║
║ ── Epic 2 ──────────────────────────────────────────         ║
║ ✓ 2-1-preconditions              loops:1  bead:ccc  10s     ║
║ ✗ 2-2-creation                   create: file not found 15s ║
║   Epic 2: 1 created, 1 failed                               ║
║                                                              ║
║ ──────────────────────────────────────────────────────       ║
║ Created: 3 | Validated: 3 | Synced: 3 | Failed: 1 | Skip: 0║
║ Total: 45s                                                   ║
╚══════════════════════════════════════════════════════════════╝
```

**Epic number extraction from story key:**
```go
// Extract epic number from story key pattern "N-M-slug"
func epicNumFromKey(key string) int {
    parts := strings.SplitN(key, "-", 3)
    if len(parts) >= 2 {
        n, _ := strconv.Atoi(parts[0])
        return n
    }
    return 0
}
```

This utility is likely already available in `internal/status/` (Entry has EpicNum field). If not, add a local helper in the output package.

**Exit code wiring in CLI commands:**

In `epic.go` and `queue.go` RunE functions:
```go
// After pipeline returns BatchResult:
app.Printer.QueueSummary(mappedResults, allKeys, result.Duration)

if result.Failed > 0 {
    return NewExitError(1)
}
return nil // exit 0
```

The precondition check already returns exit code 2 via `app.RunPreconditions()`.

**Mapping `pipeline.StoryResult` to `output.StoryResult`:**

```go
func mapStoryResults(pipelineResults []pipeline.StoryResult) []output.StoryResult {
    out := make([]output.StoryResult, len(pipelineResults))
    for i, r := range pipelineResults {
        out[i] = output.StoryResult{
            Key:             r.Key,
            Success:         r.Success,
            Duration:        r.Duration,
            FailedAt:        r.FailedAt,
            Reason:          r.Reason,
            Skipped:         r.Skipped,
            ValidationLoops: r.ValidationLoops,
            BeadID:          r.BeadID,
        }
    }
    return out
}
```

### Testing Strategy

**`output/printer_test.go` — Summary display tests:**

Use `output.NewPrinterWithWriter(&buf)` to capture output. Build `[]output.StoryResult` test data and call `QueueSummary`. Assert output contains expected substrings:

```go
tests := []struct {
    name     string
    results  []output.StoryResult
    allKeys  []string
    wantContains []string  // substrings that must appear
    wantMissing  []string  // substrings that must NOT appear
}{
    {
        name: "mixed batch with details",
        results: []output.StoryResult{
            {Key: "1-1-foo", Success: true, ValidationLoops: 2, BeadID: "abc123", Duration: 12 * time.Second},
            {Key: "1-2-bar", FailedAt: "validate", Reason: "timed out", Duration: 18 * time.Second},
            {Key: "1-3-baz", Skipped: true},
        },
        allKeys: []string{"1-1-foo", "1-2-bar", "1-3-baz"},
        wantContains: []string{
            "loops:2", "bead:abc123",           // success details
            "validate", "timed out",            // failure details
            "skipped",                          // skip indicator
            "Created: 1", "Failed: 1", "Skip: 1", // totals
        },
    },
}
```

**`cli/epic_test.go` and `cli/queue_test.go` — Exit code tests:**

Test the exit code mapping by controlling `Pipeline.RunEpic()` / `RunQueue()` return values through mock executors:
- All stories succeed → exit 0
- One story fails → exit 1
- Precondition fails → exit 2 (already tested)

Use `App` with mock dependencies. Construct `Pipeline` with mock executors that produce known `BatchResult` outcomes.

**Epic-grouped display test:**

Build results spanning epic 1 and epic 2, call the summary method, assert:
- "Epic 1" header appears
- "Epic 2" header appears
- Per-epic subtotals appear
- Grand totals appear

### Lipgloss Style Conventions

Use the existing style definitions from `internal/output/styles.go`:
- `successStyle` — green bold for checkmarks and success headers
- `errorStyle` — red bold for X marks and failure headers
- `mutedStyle` — gray for skipped indicators, secondary info
- `summaryStyle` — double-border box for summary sections
- `labelStyle` — purple bold for emphasis (epic headers)
- Icons: `iconSuccess` (checkmark), `iconError` (X), `iconPending` (circle)
- Skip icon: `↷` (already used in existing `QueueSummary` for skipped stories)

### Dependencies

- No new external dependencies
- stdlib: `fmt`, `strings`, `strconv`, `time`
- Internal: `internal/output` (Printer, styles), `internal/pipeline` (BatchResult, StoryResult), `internal/cli` (ExitError, App)
- Test: `github.com/stretchr/testify` (already in go.mod)

### Project Structure Notes

Files modified by this story:
```
internal/output/
    printer.go       # Enrich StoryResult, enhance QueueSummary, add epic-grouped display
    printer_test.go  # Tests for enhanced summary output
internal/cli/
    epic.go          # Add exit code mapping + summary display call (file from 3-1)
    queue.go         # Add exit code mapping + summary display call (file from 3-2)
    epic_test.go     # Exit code tests (file from 3-1)
    queue_test.go    # Exit code tests (file from 3-2)
```

No new packages. No new files beyond what's listed. If `epic.go` or `queue.go` already have basic summary display, modify them to use the enhanced methods.

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 3, Story 3.4]
- [Source: _bmad-output/planning-artifacts/architecture.md - Result Type Contracts]
- [Source: _bmad-output/planning-artifacts/architecture.md - Exit code semantics]
- [Source: _bmad-output/planning-artifacts/architecture.md - Package Architecture]
- [Source: _bmad-output/planning-artifacts/architecture.md - Reporting & Output]
- [Source: _bmad-output/planning-artifacts/architecture.md - Data Flow]
- [Source: _bmad-output/planning-artifacts/prd.md - FR35, FR36]
- [Source: _bmad-output/planning-artifacts/prd.md - Journey 3: Full Queue Processing]
- [Source: _bmad-output/planning-artifacts/prd.md - Scripting Support]
- [Source: internal/output/printer.go - QueueSummary, QueueHeader, StoryResult]
- [Source: internal/output/styles.go - lipgloss styles, icons, color palette]
- [Source: internal/pipeline/results.go - StepResult, StoryResult, BatchResult]
- [Source: internal/pipeline/pipeline.go - Run(), runStep()]
- [Source: internal/pipeline/steps.go - Pipeline struct, PipelineOption, dryRun/verbose fields]
- [Source: internal/cli/root.go - App struct, NewRootCommand, ExecuteResult]
- [Source: internal/cli/errors.go - ExitError, NewExitError, IsExitError]
- [Source: internal/cli/run.go - exit code pattern reference]

### Previous Story Intelligence

**From Story 2-5 (Run Command — review):**
- `Pipeline.Run()` returns `(StoryResult, error)`. Infrastructure errors return `error`; operational failures return `StoryResult{Success: false}`.
- CLI exit code pattern: `RunPreconditions()` for exit 2, `NewExitError(1)` for operational failures, nil for success (exit 0).
- `output.NewPrinterWithWriter(&buf)` pattern for test output capture is well-established.
- Pipeline tests use injectable step functions via `runPipeline()` — 17 tests cover retry, skip, failure, and infrastructure error paths.

**From Story 2-1 (Preconditions — done):**
- `ExitError{Code: 2}` for precondition failures is the established pattern. Story 3-4 reuses this — no changes needed for exit code 2.

**Key pattern from Epic 2 testing:**
- Table-driven tests with `testify/assert` (soft assertions) and `testify/require` (fatal assertions for setup).
- `t.TempDir()` for isolated filesystem. Mock executors for subprocess simulation.
- Output tests assert substring presence — not exact format matches.

### Git Intelligence

Most recent commit: `12c80ae refactor: rename project from bmad-automate to story-factory`. Module path is `story-factory`. All imports use the new module name.

The `QueueSummary` method was carried from legacy code and has basic counting logic. It doesn't show validation loops or bead IDs — that's what this story adds.

No batch-related commits exist yet (Epic 3 stories are all in backlog as of the current codebase state).

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — clean implementation with no blocking issues.

### Completion Notes List

- Enriched `output.StoryResult` with `ValidationLoops`, `BeadID`, `Reason` fields and added `BatchCounts` type for pre-computed batch totals
- Replaced legacy `QueueSummary` (which showed basic Completed/Skipped/Failed/Remaining counts) with structured per-story detail: validation loops, bead IDs, failure reasons, and `Created|Validated|Synced|Failed|Skip` totals
- Added `BatchSummary` method to `Printer` interface for epic-grouped display with per-epic headers, subtotals, and grand totals; used by `queue` command
- Added `mapStoryResults` and `mapBatchCounts` helpers in CLI layer to convert `pipeline.StoryResult` → `output.StoryResult` without making `output/` import `pipeline/`
- Updated `epic.go` to call `QueueSummary` (flat) and `queue.go` to call `BatchSummary` (grouped)
- Exit codes were already wired correctly from stories 3-1/3-2 (0=success, 1=failure, 2=precondition)
- Added 7 new printer tests covering all-success, mixed, all-failed, all-skipped, epic-grouped, and epic-grouped with skipped scenarios
- Added 4 new CLI exit code tests (exit 0 and exit 1 for both epic and queue commands)
- All tests pass: `just check` clean (fmt + vet + 38 tests across affected packages)

### File List

- internal/output/printer.go (modified — enriched StoryResult, added BatchCounts, rewrote QueueSummary, added BatchSummary + helpers)
- internal/output/printer_test.go (modified — updated 2 existing tests, added 7 new summary tests)
- internal/cli/epic.go (modified — added mapStoryResults/mapBatchCounts helpers, updated summary call)
- internal/cli/epic_test.go (modified — added ExitCode0OnAllSuccess and ExitCode1OnPartialFailure tests)
- internal/cli/queue.go (modified — switched to BatchSummary, removed unused output import)
- internal/cli/queue_test.go (modified — added ExitCode0OnAllSuccess and ExitCode1OnPartialFailure tests)

### Change Log

- 2026-04-07: Implemented structured batch summary with per-story details, epic-grouped display, and exit code verification tests
