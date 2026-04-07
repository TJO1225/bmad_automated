# Story 2.1: Precondition Verification

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an operator,
I want the tool to verify that all required dependencies are available before processing,
so that I get immediate, clear feedback if my environment isn't ready instead of a cryptic failure mid-pipeline.

## Acceptance Criteria

1. **Given** the `bd` CLI is not installed or not on PATH
   **When** the operator runs any processing command
   **Then** the tool prints a clear error message identifying `bd` as missing
   **And** exits with code 2

2. **Given** `sprint-status.yaml` does not exist at the expected project location
   **When** the operator runs any processing command
   **Then** the tool prints a clear error message identifying the missing file and expected path
   **And** exits with code 2

3. **Given** BMAD agent files are not present in the project
   **When** the operator runs any processing command
   **Then** the tool prints a clear error message identifying the missing agent files
   **And** exits with code 2

4. **Given** all preconditions are met (`bd` on PATH, YAML exists, agent files present)
   **When** the operator runs any processing command
   **Then** precondition checks complete in under 2 seconds (NFR10)
   **And** processing proceeds normally

## Tasks / Subtasks

- [x] Task 1: Create `internal/pipeline/errors.go` — sentinel errors (AC: #1, #2, #3)
  - [x] Define `ErrPreconditionFailed` sentinel error
  - [x] Add `PreconditionError` struct with `Check string` (which check failed) and `Detail string` (human-readable explanation)
  - [x] `PreconditionError` must implement `error` interface and wrap `ErrPreconditionFailed` via `Unwrap()`
- [x] Task 2: Create `internal/pipeline/results.go` — result types (AC: all)
  - [x] Define `StepResult` struct: `Name string`, `Success bool`, `Reason string`, `Duration time.Duration`, `ValidationLoops int`, `BeadID string`
  - [x] Define `StoryResult` struct: `Key string`, `Success bool`, `FailedAt string`, `Reason string`, `Duration time.Duration`, `ValidationLoops int`, `BeadID string`, `Skipped bool`
  - [x] Define `BatchResult` struct: `Stories []StoryResult`, `EpicNum int`, `Duration time.Duration`, `Created int`, `Validated int`, `Synced int`, `Failed int`, `Skipped int`
- [x] Task 3: Create `internal/pipeline/preconditions.go` — check functions (AC: #1, #2, #3, #4)
  - [x] `CheckBdCLI() error` — uses `exec.LookPath("bd")` to verify `bd` is on PATH; returns `PreconditionError` on failure
  - [x] `CheckSprintStatus(projectDir string) error` — verifies `sprint-status.yaml` exists at the resolved path using `status.DefaultStatusPath`; returns `PreconditionError` with expected path in detail
  - [x] `CheckBMADAgents(projectDir string) error` — verifies BMAD agent files exist (check for `.claude/skills/bmad-create-story/` directory); returns `PreconditionError` on failure
  - [x] `CheckAll(projectDir string) error` — runs all three checks, returns the FIRST failure as a `PreconditionError`; returns nil if all pass
- [x] Task 4: Create `internal/pipeline/preconditions_test.go` — unit tests (AC: #1, #2, #3, #4)
  - [x] Test `CheckBdCLI` — cannot easily test PATH presence; test via `CheckAll` integration or document as requiring real environment
  - [x] Test `CheckSprintStatus` — use `t.TempDir()` with and without a `sprint-status.yaml` file
  - [x] Test `CheckBMADAgents` — use `t.TempDir()` with and without `.claude/skills/bmad-create-story/` directory
  - [x] Test `CheckAll` — success when all present, failure returns first failing check
  - [x] Test error wrapping — verify `errors.Is(err, ErrPreconditionFailed)` works for all check failures
  - [x] Use table-driven tests with `testify/assert` and `testify/require`
- [x] Task 5: Wire preconditions into CLI — `internal/cli/preconditions.go` (AC: #1, #2, #3)
  - [x] Create `preconditions.go` in `internal/cli/`
  - [x] Add `(app *App) RunPreconditions() error` method that calls `pipeline.CheckAll()` with the project directory
  - [x] On failure, print the error via `app.Printer` and return `NewExitError(2)` (exit code 2 per FR31)
  - [x] This method will be called as the first step of every processing command (run, epic, queue, create-story, validate-story, sync-to-beads) — commands don't exist yet, so just expose the method for now
- [x] Task 6: Create `internal/cli/preconditions_test.go` — CLI integration tests (AC: #1, #2, #3, #4)
  - [x] Test `RunPreconditions` with all checks passing (using t.TempDir with proper directory structure)
  - [x] Test `RunPreconditions` with missing `bd` (harder — may need to test at the pipeline level)
  - [x] Test exit code 2 on precondition failure
  - [x] Use `output.NewPrinterWithWriter(buf)` to capture output and verify error messages
- [x] Task 7: Verify `just check` passes (fmt + vet + test)

### Review Findings

- [x] [Review][Patch] Sprint-status precondition accepts a directory at the YAML path — [internal/pipeline/preconditions.go] — fixed: require regular file via `Mode().IsRegular()`
- [x] [Review][Patch] BMAD agents precondition accepts a regular file instead of the expected skill directory — [internal/pipeline/preconditions.go] — fixed: require directory via `IsDir()`

## Dev Notes

### What This Story IS

Create the precondition verification system that runs before any pipeline processing. This includes:
1. The `internal/pipeline/` package foundation (errors, result types)
2. Precondition check functions for `bd` CLI, sprint-status.yaml, and BMAD agent files
3. CLI integration to run preconditions and exit with code 2 on failure

### What This Story is NOT

- Do NOT implement any pipeline step logic (stepCreate, stepValidate, stepSync) — that's Stories 2.2–2.4
- Do NOT create the `Pipeline` struct — that's Story 2.5
- Do NOT create any CLI subcommands (run, epic, queue, etc.) — those come in later stories
- Do NOT create the `internal/beads/` package — that's Story 2.4
- Do NOT modify `internal/status/`, `internal/claude/`, or `internal/config/`
- Do NOT add the `Printer` interface to `internal/pipeline/` yet — the architecture calls for it, but it's not needed until the pipeline steps exist. The current `output.Printer` is sufficient for precondition error display via the CLI layer.

### Architecture Constraints

**New package: `internal/pipeline/`** — This story creates the package with only `errors.go`, `results.go`, and `preconditions.go`. The `Pipeline` struct, step methods, and batch logic come in later stories.

**Package dependency rules:**
- `pipeline/` depends on `status/` (for `DefaultStatusPath` constant) — leaf imports only
- `pipeline/` does NOT import `claude/`, `beads/`, `output/`, or `config/` in this story
- `cli/` depends on `pipeline/` (for `CheckAll` and `PreconditionError`)
- `cli/` depends on `output/` (for `Printer` to display errors)

**Error representation:**
- `ErrPreconditionFailed` is a sentinel error (for `errors.Is()` checking)
- `PreconditionError` is a struct wrapping the sentinel with check-specific details
- Infrastructure failures → `error` return; this is an infrastructure concern, not an operational outcome

**Exit code 2** — FR31 mandates exit code 2 for precondition failures. This is distinct from exit code 1 (partial failure in batch) and 0 (success). Use `NewExitError(2)` from `internal/cli/errors.go`.

### Result Types — Define Now, Use Later

The architecture requires `StepResult`, `StoryResult`, and `BatchResult` in `internal/pipeline/results.go`. Define them in this story so the package foundation is complete, even though they won't be used until Stories 2.2+. This prevents future stories from dealing with package creation overhead.

```go
// internal/pipeline/results.go

type StepResult struct {
    Name             string
    Success          bool
    Reason           string
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

### Precondition Check Details

**Check 1: `bd` CLI on PATH (FR28)**
- Use `exec.LookPath("bd")` — returns error if not found
- Error message: `"bd CLI not found on PATH — install Gastown Beads CLI"`

**Check 2: sprint-status.yaml exists (FR29)**
- Build path: `filepath.Join(projectDir, status.DefaultStatusPath)`
- Use `os.Stat()` to check existence
- Error message must include the expected path: `"sprint-status.yaml not found at <path>"`
- `status.DefaultStatusPath` is `"_bmad-output/implementation-artifacts/sprint-status.yaml"` — reuse this constant, do not hardcode

**Check 3: BMAD agent files present (FR30)**
- Check for `.claude/skills/bmad-create-story/` directory existence in the project root
- This is the minimum signal that BMAD is installed — the create-story skill is required for pipeline operation
- Use `os.Stat()` on `filepath.Join(projectDir, ".claude/skills/bmad-create-story")`
- Error message: `"BMAD agent files not found — expected .claude/skills/bmad-create-story/ in project"`

**`CheckAll` composition:**
- Run checks in order: bd CLI → sprint-status.yaml → BMAD agents
- Return on FIRST failure (fail-fast, per FR31)
- Return nil if all pass
- All checks are filesystem/PATH lookups — easily under 2 seconds (NFR10)

### Project Directory Handling

The `projectDir` parameter flows from the `--project-dir` CLI flag (defaulting to CWD). For this story:
- `CheckSprintStatus` and `CheckBMADAgents` accept `projectDir string`
- `CheckBdCLI` does not need projectDir (PATH is global)
- `CheckAll` accepts `projectDir string` and passes it through
- In the CLI layer, `RunPreconditions` resolves the project directory (CWD for now, `--project-dir` flag comes in Story 3.3)

### Current Code State

**`internal/cli/root.go`:**
- `App` struct with `Config`, `Executor`, `Printer`, `StatusReader` fields
- `NewApp(cfg)` wires production dependencies
- `NewRootCommand(app)` creates root Cobra command with no subcommands
- `ExitError` type in `errors.go` with `NewExitError(code)` and `IsExitError(err)`

**`internal/status/reader.go`:**
- `DefaultStatusPath = "_bmad-output/implementation-artifacts/sprint-status.yaml"` — reuse this constant
- `Reader` struct with `basePath` field
- Query methods: `Read()`, `BacklogStories()`, `StoriesByStatus()`, `StoriesForEpic()`, `StoryByKey()`, `ResolveStoryLocation()`

**`internal/output/printer.go`:**
- `Printer` interface with `SessionStart()`, `SessionEnd()`, `StepStart()`, `StepEnd()`, etc.
- `NewPrinterWithWriter(w io.Writer)` for test output capture
- Existing `StepResult` and `StoryResult` types in `output/` — NOTE: the architecture says these should move to `pipeline/` eventually, but for this story just define the new ones in `pipeline/` and don't touch `output/`

**No `internal/pipeline/` exists yet** — this story creates it.

### Testing Strategy

**`preconditions_test.go` in `internal/pipeline/`:**
- Use `t.TempDir()` to create controlled filesystem environments
- Create/omit `sprint-status.yaml` and `.claude/skills/bmad-create-story/` to test each check
- `CheckBdCLI`: hard to mock PATH lookups without changing the real PATH. Options:
  - Accept that `bd` may or may not be on PATH in CI — make the test skip if `bd` is found (or not found)
  - OR: Test `CheckAll` in a temp dir where sprint-status and agents exist, and only `bd` is potentially missing
  - Recommended: Test the error type/wrapping in a unit test where you call the function and assert the error type regardless of the outcome
- Table-driven for `CheckSprintStatus` and `CheckBMADAgents` with present/absent fixture dirs
- Verify `errors.Is(err, ErrPreconditionFailed)` for all failures
- Verify `PreconditionError.Check` and `PreconditionError.Detail` contain meaningful values

**`preconditions_test.go` in `internal/cli/`:**
- Use `output.NewPrinterWithWriter(&buf)` to capture output
- Construct `App` directly with mock dependencies
- Verify `RunPreconditions` returns `ExitError{Code: 2}` on failure
- Verify printed output includes the precondition failure message

### Dependencies

- No new external dependencies
- stdlib: `os/exec` (for `LookPath`), `os` (for `Stat`), `path/filepath`, `errors`, `fmt`, `time`
- Internal: `internal/status` (for `DefaultStatusPath` constant only)
- Test: `github.com/stretchr/testify` (already in go.mod)

### Project Structure Notes

New files created by this story:
```
internal/pipeline/
    errors.go          # ErrPreconditionFailed, PreconditionError
    results.go         # StepResult, StoryResult, BatchResult
    preconditions.go   # CheckBdCLI, CheckSprintStatus, CheckBMADAgents, CheckAll
    preconditions_test.go
internal/cli/
    preconditions.go   # App.RunPreconditions()
    preconditions_test.go
```

No existing files modified (except possibly `cli/root.go` if `RunPreconditions` needs to be called from the root command — but since no subcommands exist yet, it's a standalone method).

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 2, Story 2.1]
- [Source: _bmad-output/planning-artifacts/architecture.md - Package: internal/pipeline/]
- [Source: _bmad-output/planning-artifacts/architecture.md - Error Representation]
- [Source: _bmad-output/planning-artifacts/architecture.md - Result Type Contracts]
- [Source: _bmad-output/planning-artifacts/architecture.md - Precondition Verification: internal/cli/preconditions.go]
- [Source: _bmad-output/planning-artifacts/prd.md - FR28, FR29, FR30, FR31, NFR10]
- [Source: internal/status/reader.go - DefaultStatusPath constant]
- [Source: internal/cli/errors.go - ExitError, NewExitError, IsExitError]
- [Source: internal/output/printer.go - Printer interface, NewPrinterWithWriter]

### Previous Story Intelligence

From Story 1.3 (sprint-status-queries-and-path-resolution):
- **`DefaultStatusPath`** constant is `"_bmad-output/implementation-artifacts/sprint-status.yaml"` — reuse this for the sprint-status existence check. Do NOT hardcode the path.
- **Node API pattern** — The status Reader uses yaml.v3 Node API. Not relevant for preconditions (we only check file existence, not parse it).
- **`status.NewReader("")`** in `cli/root.go` — the basePath defaults to empty string (current working directory). The precondition check should use the same path resolution: `filepath.Join(projectDir, status.DefaultStatusPath)`.
- **Test fixture pattern** — Tests use `t.TempDir()` and `testdata/` fixtures. For preconditions, `t.TempDir()` is more appropriate since we're testing filesystem presence, not file content.
- **All tests pass with `just check`** — Maintain this standard. Run `just check` as the final verification.

From Story 1.2 (sprint-status-yaml-parser):
- **Leaf package principle** — `internal/status/` imports only stdlib + yaml.v3. The new `internal/pipeline/` should follow similar discipline: only import what's needed.
- **testify conventions** — `assert` for soft checks, `require` for fatal preconditions. Follow the same pattern.

### Git Intelligence

Recent commits show the project was just renamed from `bmad-automate` to `story-factory` (commit `12c80ae`). The module path is `story-factory`. All import paths already reflect the new name. No legacy naming concerns.

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — clean implementation with no blocking issues.

### Completion Notes List

- Created `internal/pipeline/` package with errors, result types, and precondition checks
- `ErrPreconditionFailed` sentinel + `PreconditionError` struct with `Check`/`Detail` fields and proper `Unwrap()` for `errors.Is()` support
- `StepResult`, `StoryResult`, `BatchResult` defined per architecture spec (used in later stories)
- Three precondition checks: `CheckBdCLI` (exec.LookPath), `CheckSprintStatus` (os.Stat at DefaultStatusPath), `CheckBMADAgents` (os.Stat at .claude/skills/bmad-create-story/)
- `CheckAll` runs all three in order, fail-fast on first failure
- `App.RunPreconditions()` in CLI layer calls `CheckAll` with CWD, prints error via Printer, returns `ExitError{Code: 2}`
- 9 pipeline unit tests + 4 CLI integration tests, all passing
- `just check` passes (fmt + vet + test) with zero regressions
- Code review (2026-04-06): sprint-status path must be a regular file; BMAD skill path must be a directory

### Change Log

- 2026-04-06: Implemented story 2-1 — precondition verification system with pipeline package foundation
- 2026-04-06: Code review follow-up — `CheckSprintStatus` / `CheckBMADAgents` type checks + tests

### File List

New files:
- internal/pipeline/errors.go
- internal/pipeline/results.go
- internal/pipeline/preconditions.go
- internal/pipeline/preconditions_test.go
- internal/cli/preconditions.go
- internal/cli/preconditions_test.go
