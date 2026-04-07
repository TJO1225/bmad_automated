# Story 3.2: Queue Command & Cross-Epic Ordering

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an operator,
I want to process the entire backlog across all epics with a single command,
so that I can fire-and-forget the full sprint's story processing.

## Acceptance Criteria

1. **Given** a backlog with stories across Epic 1 (2 stories), Epic 2 (3 stories), and Epic 3 (1 story)
   **When** the operator runs `story-factory queue`
   **Then** epics are processed in numeric order: Epic 1, then Epic 2, then Epic 3
   **And** within each epic, stories are processed in key order
   **And** a single `BatchResult` contains all 6 `StoryResult` entries

2. **Given** Epic 1 stories are already processed (non-backlog) but Epic 2 has backlog stories
   **When** the operator runs `story-factory queue`
   **Then** Epic 1 stories are skipped
   **And** Epic 2 stories are processed normally

3. **Given** no backlog stories exist anywhere
   **When** the operator runs `story-factory queue`
   **Then** the tool reports that no backlog stories were found
   **And** exits with code 0

## Tasks / Subtasks

- [x] Task 1: Add `RunQueue()` method to Pipeline struct in `internal/pipeline/batch.go` (AC: #1, #2, #3)
  - [x] Accept `ctx context.Context` parameter (no args — processes entire backlog)
  - [x] Create a fresh `status.Reader` from `p.projectDir` and call `BacklogStories()` to get all backlog stories sorted by epic→story order
  - [x] Handle empty backlog: return `BatchResult` with zero stories and `Duration` set
  - [x] Iterate over backlog stories, calling `p.Run(ctx, key)` for each
  - [x] Re-read sprint-status.yaml before each story by constructing a fresh `status.Reader` (BMAD modifies status between operations)
  - [x] Skip stories no longer in backlog on re-read (resumable batch support)
  - [x] Collect each `StoryResult` into `BatchResult.Stories`
  - [x] Continue processing remaining stories even when one fails (no abort on single failure)
  - [x] Populate `BatchResult` summary fields: `Created`, `Validated`, `Synced`, `Failed`, `Skipped`, `Duration`
  - [x] Track total `Duration` from start to completion
- [x] Task 2: Add `RunEpic()` method to Pipeline struct in `internal/pipeline/batch.go` (AC: #1, #2)
  - [x] Accept `ctx context.Context` and `epicNum int` parameters
  - [x] Create a fresh `status.Reader` and call `StoriesForEpic(epicNum)` to get stories for the epic
  - [x] Filter to only `StatusBacklog` entries (other statuses skipped)
  - [x] Handle empty result: return `BatchResult` with `EpicNum` set and zero stories
  - [x] Iterate stories calling `p.Run(ctx, key)` for each, re-reading status between stories
  - [x] Collect results, populate summary fields, set `BatchResult.EpicNum`
- [x] Task 3: Create `internal/cli/queue.go` — `queue` CLI command (AC: #1, #2, #3)
  - [x] Add `queue` Cobra command accepting no positional args
  - [x] Run preconditions first via `app.RunPreconditions()`
  - [x] Construct Pipeline with all dependencies (same pattern as `run.go`)
  - [x] Call `pipeline.RunQueue(ctx)`
  - [x] Display results via `Printer.QueueSummary()` and `Printer.QueueHeader()`
  - [x] Exit code 0 if all succeed or empty backlog, exit code 1 if any story failed
  - [x] Register command in `NewRootCommand()` in `root.go`
- [x] Task 4: Update `internal/cli/epic.go` — `epic` CLI command (AC: #1, #2)
  - [x] Add `epic <N>` Cobra command accepting exactly 1 positional arg (epic number)
  - [x] Parse epic number from string arg, validate it is a positive integer
  - [x] Run preconditions, construct Pipeline, call `pipeline.RunEpic(ctx, epicNum)`
  - [x] Display results via `Printer.QueueSummary()` and `Printer.QueueHeader()`
  - [x] Register command in `NewRootCommand()` in `root.go`
- [x] Task 5: Create `internal/pipeline/batch_test.go` — unit tests for RunQueue and RunEpic (AC: #1–#3)
  - [x] Test RunQueue happy path: multiple epics with backlog stories, all processed in order
  - [x] Test RunQueue with mixed statuses: some stories skipped, some processed
  - [x] Test RunQueue empty backlog: returns BatchResult with zero stories
  - [x] Test RunQueue with failure: one story fails, rest continue
  - [x] Test RunEpic happy path: single epic, stories in order
  - [x] Test RunEpic with no backlog stories for epic: empty BatchResult
  - [x] Test RunEpic with mixed statuses within epic
  - [x] Test cross-epic ordering: verify Epic 1 before Epic 2 before Epic 3
  - [x] Use injectable run functions via `runQueue`/`runEpic` (same testability pattern as `runPipeline`)
  - [x] Use fixture sprint-status.yaml in `t.TempDir()` with `status.NewReader()`
  - [x] Table-driven with `testify/assert` and `testify/require`
- [x] Task 6: Create `internal/cli/queue_test.go` and `internal/cli/epic_test.go` — CLI integration tests
  - [x] Test `queue` command wiring (registered, accepts no args)
  - [x] Test `epic` command wiring (registered, accepts exactly 1 arg)
  - [x] Test `epic` rejects non-numeric argument
  - [x] Test precondition failure returns exit code 2 for both commands
- [x] Task 7: Verify `just check` passes (fmt + vet + test)

### Review Findings

- [x] [Review][Patch] RunEpic should iterate backlog-only rows (Task 2, Dev Notes) — `runEpic` passes every `StoriesForEpic` row into `runEpicStories` and relies on `Pipeline.Run` to skip non-backlog, so `BatchResult.Stories` includes one entry per epic row (all skipped) instead of an empty batch when the epic has no backlog work. This diverges from the story’s “filter to backlog” / “empty BatchResult with zero stories” tasks and from the Dev Agent Record claim that RunEpic pre-filters backlog. [`internal/pipeline/batch.go`](internal/pipeline/batch.go) (`runEpic`, `runEpicStories`). **Fixed:** `runEpic` filters to backlog-only and delegates to `runBatch`; removed `runEpicStories`.

- [x] [Review][Patch] Align epic batch UX when there is no backlog — With only non-backlog rows, the CLI shows `QueueHeader(0, …)` while the pipeline still prints `Processing epic N: <len(allStories)> stories` and walks every row, which reads as inconsistent. Resolving the backlog-only iteration above fixes the primary mismatch; adjust printer strings if any edge case remains. **Fixed:** no `Processing epic` line when there is no backlog work; message uses backlog count when processing.

## Dev Notes

### What This Story IS

Implement `RunQueue()` and `RunEpic()` batch methods on Pipeline, plus the `queue` and `epic` CLI commands. This story adds the ability to process multiple stories in sequence — across all epics (`queue`) or within a single epic (`epic`).

### What This Story is NOT

- Do NOT implement `--dry-run`, `--verbose`, or `--project-dir` flags — that's Story 3-3
- Do NOT implement structured summary reporting or exit code refinement — that's Story 3-4 (the Printer methods for summary display already exist; just call them)
- Do NOT modify step implementations (`StepCreate`, `StepValidate`, `stepSync`) — those exist from Epic 2
- Do NOT modify `Pipeline.Run()` or `runStep()` — those exist from Story 2-5
- Do NOT modify `internal/status/`, `internal/claude/`, `internal/beads/`, or `internal/config/`
- Do NOT add parallel processing — sequential only (MVP, FR23)

### Architecture Constraints

**Batch iteration pattern** — From architecture doc:

```
RunQueue() iterates over all backlog stories across all epics, processing them sequentially.
RunEpic() iterates over all backlog stories in a single epic.
Both call Run() per story with YAML re-reads between.
```

**Cross-epic ordering (FR23):** Epics processed in numeric order — this is already guaranteed by `status.Reader.BacklogStories()` which sorts by `EpicNum` then `StoryNum`.

**YAML re-read between stories (FR24):** BMAD modifies `sprint-status.yaml` after each `create-story` invocation. Before processing each story, the batch must re-read the status file to get fresh state. Use `status.NewReader(p.projectDir)` to create a fresh reader (the Reader re-reads from disk on every call, so a fresh instance isn't strictly required, but it documents intent).

**Resumable batches (FR25):** Stories no longer in `backlog` on re-read are skipped. This enables stopping and re-running — already-processed stories are automatically skipped.

**No-abort on failure (FR26):** A single story failure does NOT abort the batch. The failed story gets `Success: false` in its `StoryResult`, and the batch continues to the next story.

**Package dependency rules:**
- `pipeline/` uses `status.Reader` for story queries (already injected via `WithStatus`)
- `cli/` wires dependencies and calls `pipeline.RunQueue()` / `pipeline.RunEpic()`
- `output.Printer` already has `QueueHeader()`, `QueueSummary()`, `QueueStoryStart()` methods

**Error vs operational failure:**
- `BacklogStories()` or `StoriesForEpic()` returning an `error` = infrastructure failure → bubble up
- `Run()` returning `StoryResult{Success: false}` = operational failure → record in BatchResult, continue
- `Run()` returning an `error` = infrastructure failure → record as failed, continue batch (don't abort)

### Prerequisite: Story 3-1 (Epic Batch Command)

Story 3-1 creates the `epic` command and `RunEpic()`. However, since 3-1 is still in `backlog`, this story MUST implement both `RunEpic()` and `RunQueue()` together. The queue command depends on iterating epics, and `RunQueue()` may delegate to `RunEpic()` internally, or iterate stories directly via `BacklogStories()`.

**Recommended approach:** Implement `RunQueue()` using `BacklogStories()` directly (returns all backlog stories across all epics, already sorted by epic→story order). Implement `RunEpic()` using `StoriesForEpic(n)` filtered to backlog status. Both are independent — `RunQueue()` does NOT call `RunEpic()` internally.

### RunQueue() Implementation

```go
func (p *Pipeline) RunQueue(ctx context.Context) (BatchResult, error) {
    start := time.Now()
    
    // Get all backlog stories (sorted by epic then story number)
    stories, err := p.status.BacklogStories()
    if err != nil {
        return BatchResult{}, err
    }
    
    var result BatchResult
    for _, entry := range stories {
        // Re-read fresh status before each story
        freshReader := status.NewReader(p.projectDir)
        freshEntry, err := freshReader.StoryByKey(entry.Key)
        if err != nil || freshEntry.Status != status.StatusBacklog {
            // Story no longer backlog — skip
            result.Stories = append(result.Stories, StoryResult{Key: entry.Key, Skipped: true})
            result.Skipped++
            continue
        }
        
        storyResult, err := p.Run(ctx, entry.Key)
        if err != nil {
            // Infrastructure error — record as failed, continue batch
            result.Stories = append(result.Stories, StoryResult{
                Key:      entry.Key,
                FailedAt: "infrastructure",
                Reason:   err.Error(),
            })
            result.Failed++
            continue
        }
        
        result.Stories = append(result.Stories, storyResult)
        // Update counters based on result...
    }
    
    result.Duration = time.Since(start)
    return result, nil
}
```

**Counter logic:** After each `storyResult`:
- `Skipped` → `result.Skipped++`
- `Success` → `result.Created++`, `result.Validated++`, `result.Synced++`
- `!Success && !Skipped` → `result.Failed++`

### RunEpic() Implementation

Same pattern but uses `StoriesForEpic(epicNum)` and filters to backlog-only. Sets `BatchResult.EpicNum`.

### CLI Command Patterns

Follow the established pattern from `run.go`:

**`queue` command (`internal/cli/queue.go`):**
```go
func newQueueCommand(app *App) *cobra.Command {
    return &cobra.Command{
        Use:   "queue",
        Short: "Process all backlog stories across all epics",
        Args:  cobra.NoArgs,
        RunE: func(cmd *cobra.Command, args []string) error {
            // 1. RunPreconditions()
            // 2. Construct pipeline
            // 3. Call RunQueue()
            // 4. Display results
            // 5. Exit code
        },
    }
}
```

**`epic` command (`internal/cli/epic.go`):**
```go
func newEpicCommand(app *App) *cobra.Command {
    return &cobra.Command{
        Use:   "epic <N>",
        Short: "Process all backlog stories in an epic",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            epicNum, err := strconv.Atoi(args[0])
            if err != nil {
                return fmt.Errorf("invalid epic number: %s", args[0])
            }
            // ... same pattern as queue
        },
    }
}
```

**Exit code semantics (from architecture):**
- `0` — all stories succeeded (or empty backlog)
- `1` — partial failure (at least one story failed)
- `2` — precondition failure (before processing)

### Testing Strategy

**Pipeline tests (`internal/pipeline/batch_test.go`):**

The existing `pipeline_test.go` already uses injectable `stepFunc` via `runPipeline()`. For batch tests, the challenge is that `RunQueue()` and `RunEpic()` call `p.Run()` which calls real step methods. Two approaches:

1. **Fixture-based:** Create fixture sprint-status.yaml and configure mock executors to produce desired outcomes. This tests the full integration.

2. **Preferred: Extract batch iteration logic.** Create internal helpers like `runBatch(ctx, stories)` that accept a list of story keys and call `p.Run()` per story. Test the batch logic by mocking `Run()` — but `Run()` isn't an interface method.

**Best approach for this codebase:** Use the existing `runPipeline()` pattern. Create `runQueueWith(ctx, stories, runFn)` where `runFn` replaces `p.Run()`. In production, pass `p.Run`; in tests, pass a mock function. This follows the established testability pattern.

Alternatively, since `Run()` reads from `p.status` which reads from disk, set up fixture YAML files and configure mock executors to control step behavior. The `pipeline_test.go` tests for `Run()` already demonstrate this pattern with `setupSprintStatus()` helper.

**CLI tests:** Same pattern as `run_test.go` — verify command wiring, argument validation, precondition checks.

### Existing Code to Reuse

- **`Pipeline.Run(ctx, key)`** — processes a single story, already handles retry and skip logic
- **`status.Reader.BacklogStories()`** — returns all backlog stories sorted by epic→story order
- **`status.Reader.StoriesForEpic(n)`** — returns all stories (all statuses) for an epic
- **`output.Printer.QueueHeader()`, `QueueSummary()`, `QueueStoryStart()`** — batch display methods already exist
- **`BatchResult` struct** — already defined in `results.go` with `Stories`, `EpicNum`, `Duration`, `Created`, `Validated`, `Synced`, `Failed`, `Skipped` fields
- **`StoryResult` struct** — already defined, collected per-story
- **Constructor pattern:** `NewPipeline(executor, cfg, projectDir, ...opts)` with `WithStatus`, `WithPrinter`, `WithBeads`
- **CLI pattern:** `newXxxCommand(app)` returning `*cobra.Command`, registered in `NewRootCommand()`

### Dependencies

- No new external dependencies
- stdlib: `context`, `time`, `fmt`, `strconv`
- Internal: `internal/pipeline` (Pipeline, BatchResult, StoryResult), `internal/cli` (App, ExitError), `internal/status` (Reader, Entry, StatusBacklog), `internal/output` (Printer)
- Test: `github.com/stretchr/testify` (already in go.mod)

### Project Structure Notes

New files created by this story:
```
internal/pipeline/
    batch.go          # RunQueue(), RunEpic() methods (or added to pipeline.go)
    batch_test.go     # Batch method tests
internal/cli/
    queue.go          # queue CLI command
    queue_test.go     # queue CLI tests
    epic.go           # epic CLI command
    epic_test.go      # epic CLI tests
    root.go           # MODIFY: register queue and epic subcommands
```

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 3, Story 3.2]
- [Source: _bmad-output/planning-artifacts/architecture.md - Pipeline Composition]
- [Source: _bmad-output/planning-artifacts/architecture.md - Package Architecture]
- [Source: _bmad-output/planning-artifacts/architecture.md - Error Handling & Retry]
- [Source: _bmad-output/planning-artifacts/prd.md - FR20, FR21, FR22, FR23, FR24, FR25, FR26, FR27]
- [Source: _bmad-output/planning-artifacts/prd.md - CLI Commands: queue, epic]
- [Source: internal/pipeline/pipeline.go - Run(), runPipeline(), runStep(), stepFunc]
- [Source: internal/pipeline/results.go - StepResult, StoryResult, BatchResult]
- [Source: internal/status/reader.go - BacklogStories(), StoriesForEpic(), StoryByKey()]
- [Source: internal/status/types.go - Entry, Status, StatusBacklog]
- [Source: internal/cli/run.go - newRunCommand() pattern]
- [Source: internal/cli/root.go - App struct, NewRootCommand()]
- [Source: internal/output/printer.go - QueueHeader(), QueueSummary(), QueueStoryStart()]

### Previous Story Intelligence

From Story 2-5 (full-pipeline-composition-and-run-command):
- **`Pipeline.Run(ctx, key)`** exists in `pipeline.go` with retry-once semantics via `runStep()`
- **`runPipeline(ctx, key, create, validate, sync)`** accepts injectable `stepFunc` for testability
- **`stepFunc` type** defined as `func(context.Context, string) (StepResult, error)`
- **`pipeline_test.go`** has comprehensive tests using `setupSprintStatus()` helper, mock step functions (`successStep`, `failStep`, `errorStep`), and `newTestPipeline()` helper
- **`App.BeadsExecutor`** field exists on App struct, wired in `NewApp()`
- **`run.go` CLI pattern** — constructs fresh `claude.Executor` with `WorkingDir`, creates Pipeline with all `With*` options, calls `p.Run()`, handles result/skip/error
- **Infrastructure errors from `Run()`** — bubble up as `error` return; operational failures are in `StoryResult.Success == false`

From Story 2-3 (story-validation):
- **`StepValidate()`** is exported (uppercase) — called from `validate_story.go` CLI
- **`setupStoryFile()`** helper in `steps_test.go` creates files at `_bmad-output/implementation-artifacts/<key>.md`
- **Mtime testing trick** — use `os.Chtimes()` to explicitly advance timestamps in tests

From Story 1-3 (sprint-status-queries):
- **`BacklogStories()`** returns `[]Entry` sorted by EpicNum then StoryNum — perfect for cross-epic ordering
- **`StoriesForEpic(n)`** returns ALL stories for epic (all statuses) — filter to backlog in `RunEpic()`
- **`status.NewReader(basePath)`** — pass `p.projectDir` for fresh reads. Reader re-reads from disk each time.

### Git Intelligence

Recent commits (all pre-dating Epic 2 implementation):
- `12c80ae` — refactor: rename project from bmad-automate to story-factory. Module path is `story-factory`.
- Project uses `just` as build system. Run `just check` for fmt + vet + test.
- All import paths use `story-factory/internal/...` module path.

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — clean implementation, no debugging needed.

### Completion Notes List

- Implemented `RunQueue()` and `RunEpic()` batch methods in `internal/pipeline/batch.go`
- Both methods use shared `runBatch()` helper with fresh-reader re-read before each story (FR24)
- `RunEpic()` updated from Story 3-1: now pre-filters to backlog-only stories and uses `runBatch()` with fresh-reader pattern
- `RunQueue()` uses `BacklogStories()` which guarantees cross-epic ordering by epic→story number (FR23)
- Resumable batch support: stories no longer backlog on re-read are skipped (FR25)
- No-abort on failure: infrastructure errors recorded as failed StoryResult, batch continues (FR26)
- Injectable `runFunc` pattern enables unit testing without real subprocess execution
- Created `queue` CLI command with `QueueHeader`/`QueueSummary` display
- Updated `epic` CLI command to use same `QueueHeader`/`QueueSummary` display pattern and backlog pre-filtering
- Both commands pass `DryRun` and `Verbose` flags through to Pipeline
- TestRunQueue_SkipsStoriesNoLongerBacklog verifies the fresh-reader re-read by mutating the YAML file mid-batch
- TestRunQueue_CrossEpicOrdering verifies stories out of order in YAML are processed in epic→story order
- All 50+ pipeline tests and 30+ CLI tests pass
- `just check` passes (fmt + vet + test)

### File List

- internal/pipeline/batch.go (modified — rewrote with RunQueue, runBatch, updated RunEpic to pre-filter backlog)
- internal/pipeline/batch_test.go (modified — rewrote with RunQueue tests, cross-epic ordering, resumable batch tests)
- internal/cli/queue.go (new — queue CLI command)
- internal/cli/queue_test.go (new — queue CLI tests)
- internal/cli/epic.go (modified — updated to use QueueHeader/QueueSummary display and backlog pre-filtering)
- internal/cli/epic_test.go (unchanged — existing tests still pass)
- internal/cli/root.go (modified — registered queue command)

## Change Log

- 2026-04-07: Implemented Story 3-2 — RunQueue/RunEpic batch methods, queue CLI command, epic CLI updates
