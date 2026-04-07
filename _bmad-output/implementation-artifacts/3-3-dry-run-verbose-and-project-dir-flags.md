# Story 3.3: Dry-Run, Verbose, and Project-Dir Flags

Status: done

## Story

As an operator,
I want to preview planned operations, stream real-time output, and target non-CWD projects,
so that I can verify batch plans before committing, monitor complex story processing, and work across multiple projects.

## Acceptance Criteria

1. **Given** Epic 1 has 3 backlog stories
   **When** the operator runs `story-factory epic 1 --dry-run`
   **Then** the tool reads `sprint-status.yaml` and lists planned operations per story
   **And** each planned operation shows: story key, steps that would execute (create->validate->sync)
   **And** no Claude or bd subprocess is invoked
   **And** no files are created or modified

2. **Given** the operator runs with `--verbose`
   **When** a Claude subprocess streams JSON events
   **Then** text events are forwarded to the Printer and displayed in real time
   **And** bd stdout is also displayed

3. **Given** the operator runs without `--verbose`
   **When** a Claude subprocess streams JSON events
   **Then** only progress indicators are displayed (not streaming content)

4. **Given** the operator runs `story-factory epic 1 --project-dir /home/tom/projects/other-app`
   **When** the tool resolves paths and spawns subprocesses
   **Then** `sprint-status.yaml` is read from `/home/tom/projects/other-app/`
   **And** all subprocess working directories are set to `/home/tom/projects/other-app/`
   **And** story file paths are resolved relative to that directory

## Tasks / Subtasks

- [x] Task 1: Add persistent flags to root command (AC: #1, #2, #4)
  - [x] 1.1: Add `--dry-run`, `--verbose`, `--project-dir` as persistent flags on root Cobra command
  - [x] 1.2: Add `DryRun`, `Verbose`, `ProjectDir` fields to App struct
  - [x] 1.3: Add `ResolveProjectDir()` method on App (returns flag value or os.Getwd() fallback)
- [x] Task 2: Thread --project-dir through all commands (AC: #4)
  - [x] 2.1: Replace `os.Getwd()` in `cli/run.go`, `cli/create_story.go`, `cli/validate_story.go`, `cli/preconditions.go` with `app.ResolveProjectDir()`
  - [x] 2.2: Pass `WithDryRun(app.DryRun)` and `WithVerbose(app.Verbose)` in all `pipeline.NewPipeline()` calls
- [x] Task 3: Complete dry-run support in remaining pipeline steps (AC: #1)
  - [x] 3.1: Add dry-run guard to `StepValidate()` at top of method (before status reader check)
  - [x] 3.2: Add dry-run guard to `stepSync()` method (before path resolution)
- [x] Task 4: Add verbose bd output display to `stepSync()` (AC: #2)
  - [x] 4.1: Stream real bd stdout and stderr to the Printer when verbose is true
- [x] Task 5: Write tests (AC: #1, #2, #3, #4)
  - [x] 5.1: Test flag registration and defaults on root command
  - [x] 5.2: Test projectDir fallback to os.Getwd() when flag not set
  - [x] 5.3: Test StepValidate dry-run returns success without invoking Claude
  - [x] 5.4: Test stepSync dry-run returns success without invoking bd
  - [x] 5.5: Test each command propagates flags to pipeline construction

## Dev Notes

### What Already Exists (DO NOT Rebuild)

The Pipeline struct (`internal/pipeline/steps.go:32-40`) already has `dryRun bool` and `verbose bool` fields. Pipeline options already exist:

- `WithDryRun(v bool)` at `steps.go:74-76`
- `WithVerbose(v bool)` at `steps.go:79-81`

**StepCreate** already fully implements both:
- **Dry-run** (`steps.go:91-101`): Returns `StepResult{Success: true}` with message, no subprocess
- **Verbose** (`steps.go:115-121`): Builds an `EventHandler` that forwards `event.IsText()` to `Printer.Text()`

**StepValidate** already implements verbose (`steps.go:232-239`) — same EventHandler pattern as StepCreate. DO NOT add verbose to StepValidate again.

The dry-run pattern from StepCreate is the template. Replicate it for `StepValidate()` and `stepSync()` only.

### What's Missing (This Story's Scope)

**1. CLI Flag Definitions — `internal/cli/root.go`**

No Cobra flags exist on any command. Add three persistent flags to the root command in `NewRootCommand()` (lines 110-125):

```go
rootCmd.PersistentFlags().BoolVar(&app.DryRun, "dry-run", false, "Show planned operations without executing")
rootCmd.PersistentFlags().BoolVar(&app.Verbose, "verbose", false, "Stream Claude CLI output in real time")
rootCmd.PersistentFlags().StringVar(&app.ProjectDir, "project-dir", "", "Project root directory (default: current working directory)")
```

Add fields to `App` struct (`root.go:59-74`):
```go
DryRun     bool
Verbose    bool
ProjectDir string
```

Add a `ResolveProjectDir()` method on `App` that returns `app.ProjectDir` if non-empty, otherwise `os.Getwd()`. Every command calls this inside its `RunE` (not during app construction) since persistent flags are only parsed after Cobra dispatches to the command.

**IMPORTANT timing note:** `NewApp()` runs before flags are parsed. Do NOT try to use `app.ProjectDir` inside `NewApp()`. The `status.NewReader("")` call in `NewApp()` (line 96) creates a reader with empty basePath — this is fine because each command creates its own `status.NewReader(projectDir)` with the resolved path when constructing the pipeline. Do not change `NewApp()`.

**2. Eliminate Hardcoded `os.Getwd()` Calls**

Four locations currently call `os.Getwd()` — ALL must use `app.ResolveProjectDir()` instead:

| File | Line | Current Code |
|------|------|-------------|
| `cli/run.go` | 32 | `projectDir, err := os.Getwd()` |
| `cli/create_story.go` | 30 | `projectDir, err := os.Getwd()` |
| `cli/validate_story.go` | 30 | `projectDir, err := os.Getwd()` |
| `cli/preconditions.go` | 17 | `projectDir, err := os.Getwd()` |

Replace each with:
```go
projectDir, err := app.ResolveProjectDir()
```

`RunPreconditions()` is a method on `App` — it can call `app.ResolveProjectDir()` directly to replace its internal `os.Getwd()` call.

**3. Pipeline Option Threading in Commands**

Every command constructs a Pipeline but never passes `WithDryRun` or `WithVerbose`. Add these options to every `pipeline.NewPipeline()` call:

```go
pipeline.WithDryRun(app.DryRun),
pipeline.WithVerbose(app.Verbose),
```

Locations:
- `cli/run.go:46-53` — pipeline.NewPipeline call
- `cli/create_story.go:44-50` — pipeline.NewPipeline call
- `cli/validate_story.go:55-61` — pipeline.NewPipeline call

**4. StepValidate Dry-Run — `internal/pipeline/steps.go:215`**

`StepValidate()` already has verbose support (lines 232-239). It is ONLY missing a dry-run guard.

Add dry-run guard immediately after `start := time.Now()` at line 216, before the status reader check:
```go
if p.dryRun {
    msg := "dry-run: would validate story " + key
    if p.printer != nil {
        p.printer.Text(msg)
    }
    return StepResult{
        Name:    stepNameValidate,
        Success: true,
        Reason:  msg,
    }, nil
}
```

**5. stepSync Dry-Run & Verbose — `internal/pipeline/steps.go:309-316`**

`stepSync()` is a thin method that delegates to the static `StepSync()` function. Add dry-run guard at the top of `stepSync()`, before the path resolution:
```go
if p.dryRun {
    msg := "dry-run: would sync story " + key + " to beads"
    if p.printer != nil {
        p.printer.Text(msg)
    }
    return StepResult{
        Name:    stepNameSync,
        Success: true,
        Reason:  msg,
    }, nil
}
```

Do NOT add dry-run to the static `StepSync()` function (line 326) — it has no access to Pipeline fields. Only `stepSync()` (the method called by `Run()`) needs the guard.

**6. Verbose for bd Output**

The beads `Executor.Create()` interface returns only `(beadID string, err error)` — it does not stream output. Do NOT modify the beads Executor interface.

**Approach:** Expand `stepSync()` to inline the logic from `StepSync()` instead of delegating. This gives the method access to `p.verbose` and `p.printer`. Before calling `beadsExec.Create()`, print the command. After success, print the bead ID:

```go
if p.verbose && p.printer != nil {
    p.printer.Text(fmt.Sprintf("bd create \"%s: %s\"", key, title))
}
beadID, err := p.beads.Create(ctx, key, title, acs)
// ... handle error ...
if p.verbose && p.printer != nil {
    p.printer.Text(fmt.Sprintf("bead created: %s", beadID))
}
```

Keep the static `StepSync()` function — it remains useful for direct invocation from the `sync-to-beads` CLI command (which doesn't use Pipeline).

### Architecture Compliance

**Package boundaries (DO NOT violate):**
- `pipeline/` defines `Printer` interface — never imports `output/`
- `cli/` wires `output.DefaultPrinter` as `pipeline.Printer` implementation
- All subprocess calls through `Executor` interfaces — never shell out directly
- All user output through `Printer` — no `fmt.Println` outside `internal/output/`

**Error representation:**
- Operational outcomes (dry-run messages, step results) → `StepResult{Success: true/false}`
- Infrastructure failures (filesystem errors, context cancelled) → `error` return

**Result types:**
- Steps produce `StepResult` → Pipeline maps to `StoryResult` → Batch collects to `BatchResult`
- Dry-run steps should return `StepResult{Success: true}` with the planned action in `Reason`

### Testing Standards

- **Location:** Co-located `<file>_test.go`. Fixtures in `testdata/`.
- **Pattern:** Table-driven with `testify/assert` (soft) and `testify/require` (fatal).
- **Naming:** `Test<Function>_<scenario>` (e.g., `TestStepValidate_DryRun`, `TestRootCommand_FlagDefaults`).
- **Mocks:** Use `claude.MockExecutor` (configurable events, exit codes, `RecordedPrompts`), `beads.MockExecutor` (configurable beadID, error).
- **CLI tests:** Construct `App` directly with mocks, verify command behavior. Lean on pipeline tests for logic coverage.

For dry-run tests, verify that `MockExecutor.RecordedPrompts` is empty (no subprocess was invoked). For verbose tests, verify that the handler was called by checking printer output in a `bytes.Buffer`.

For --project-dir tests, verify that the resolved path appears in:
- Status reader basePath
- Claude executor WorkingDir
- Story file path resolution

### Previous Story Intelligence

Epic 2 established these patterns that MUST be followed:
- **Retry logic lives ONLY in `Pipeline.Run()` and `runStep()`** — steps are pure
- **Post-condition verification after each step** — file exists, status changed
- **Mtime-based validation convergence** — avoids text parsing
- **SIGTERM → 5s grace → SIGKILL** for subprocess cleanup
- **Fresh `status.Reader` for post-condition checks** — never cache stale state

### Git Intelligence

Recent commits show the project was renamed from `bmad-automate` to `story-factory` (commit `12c80ae`). Module path is `story-factory`. Binary output is `./story-factory`.

### Project Structure Notes

Files to create or modify:

| File | Action |
|------|--------|
| `internal/cli/root.go` | MODIFY — Add App fields (`DryRun`, `Verbose`, `ProjectDir`), persistent flags, `ResolveProjectDir()` method. Do NOT change `NewApp()`. |
| `internal/cli/run.go` | MODIFY — Use `app.ResolveProjectDir()`, pass DryRun/Verbose to pipeline |
| `internal/cli/create_story.go` | MODIFY — Same changes as run.go |
| `internal/cli/validate_story.go` | MODIFY — Same changes as run.go |
| `internal/cli/preconditions.go` | MODIFY — Use `app.ProjectDir` instead of `os.Getwd()` |
| `internal/pipeline/steps.go` | MODIFY — Add dry-run/verbose to StepValidate and stepSync |
| `internal/cli/root_test.go` | CREATE or MODIFY — Tests for flag registration and defaults |
| `internal/pipeline/steps_test.go` | MODIFY — Add dry-run and verbose test cases |

No new packages. No new dependencies. No changes to `go.mod`.

### References

- [Source: _bmad-output/planning-artifacts/epics.md — Story 3.3 acceptance criteria]
- [Source: _bmad-output/planning-artifacts/architecture.md — Pipeline Composition, Dry-Run Threading, Output Conventions]
- [Source: _bmad-output/planning-artifacts/prd.md — FR32 (dry-run), FR33 (verbose), FR34 (project-dir)]
- [Source: internal/pipeline/steps.go:32-40 — Pipeline struct with dryRun/verbose fields]
- [Source: internal/pipeline/steps.go:74-81 — WithDryRun/WithVerbose options]
- [Source: internal/pipeline/steps.go:91-121 — Existing dry-run/verbose patterns in StepCreate]
- [Source: internal/pipeline/steps.go:215-305 — StepValidate (has verbose at 232-239, MISSING dry-run)]
- [Source: internal/pipeline/steps.go:309-316 — stepSync (MISSING dry-run and verbose)]
- [Source: internal/pipeline/steps.go:326-344 — Static StepSync function (do not modify for dry-run)]
- [Source: internal/cli/root.go:59-74 — App struct, no DryRun/Verbose/ProjectDir fields]
- [Source: internal/cli/root.go:110-125 — NewRootCommand, no persistent flags]
- [Source: internal/cli/root.go:84-105 — NewApp(), status.NewReader("") — do not change]
- [Source: internal/cli/preconditions.go:17 — os.Getwd() hardcoded]
- [Source: internal/cli/run.go:32 — os.Getwd() hardcoded]
- [Source: internal/cli/create_story.go:30 — os.Getwd() hardcoded]
- [Source: internal/cli/validate_story.go:30 — os.Getwd() hardcoded]

### Review Findings

- [x] [Review][Decision] AC2 says “bd stdout” but implementation prints synthetic lines only — Story acceptance criterion #2 requires that when Claude streams JSON, “bd stdout is also displayed.” Dev Notes and code print a constructed `bd create "…"` line and `bead created: …` instead of subprocess stdout. Choose: (A) amend AC2 / PRD to match this design, (B) extend the beads layer to capture and forward real bd stdout/stderr while keeping the executor abstraction.

- [x] [Review][Patch] Stale godoc on `NewRootCommand` still claims no subcommands [internal/cli/root.go:134]

- [x] [Review][Patch] Task 5.5 (“each command propagates flags”) is marked done but only `run` has a CLI-level dry-run propagation test; add analogous tests for `create-story`, `validate-story`, `epic`, and/or `queue`, or adjust the task checklist to match coverage.

- [x] [Review][Patch] `ResolveProjectDir` returns `ProjectDir` verbatim; a whitespace-only flag value is treated as a real path instead of falling back to `os.Getwd` [internal/cli/root.go:125-128]

**Resolution (2026-04-06):** Chose **B** — [Executor.Create] now takes `bdOut io.Writer`; [DefaultExecutor] tees stdout/stderr to `bdOut` while buffering for parse; [stepSync] uses [printerLineWriter] when verbose. Patches applied: corrected `NewRootCommand` godoc; added dry-run CLI tests for create-story, validate-story, epic, queue; `ResolveProjectDir` uses `strings.TrimSpace` on the flag.

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — clean implementation with no blocking issues.

### Completion Notes List

- Task 1: Added `DryRun`, `Verbose`, `ProjectDir` fields to App struct. Added `ResolveProjectDir()` method with `os.Getwd()` fallback. Registered three persistent flags on root Cobra command.
- Task 2: Replaced all four `os.Getwd()` calls with `app.ResolveProjectDir()` in run.go, create_story.go, validate_story.go, preconditions.go. Added `WithDryRun(app.DryRun)` and `WithVerbose(app.Verbose)` to all three `pipeline.NewPipeline()` calls.
- Task 3: Added dry-run guards to `StepValidate()` (before status reader check) and `stepSync()` (before path resolution), following the existing `StepCreate` pattern.
- Task 4: Inlined `stepSync()` logic (previously delegated to static `StepSync()`) to access Pipeline fields. Added verbose printing of bd invocation command and bead ID result. Kept static `StepSync()` unchanged for direct invocation use.
- Task 5: Created `root_test.go` with 3 tests (flag defaults, ResolveProjectDir with flag, ResolveProjectDir fallback). Added 3 pipeline tests (StepValidate dry-run, stepSync dry-run, stepSync verbose). Added 1 CLI integration test (run command --dry-run propagation).

### Change Log

- 2026-04-07: Implemented story 3-3 — dry-run, verbose, and project-dir flags
- 2026-04-06: Code review follow-up — real bd stdout/stderr streaming (verbose), godoc, `ResolveProjectDir` trim, CLI dry-run propagation tests

### File List

- `internal/cli/root.go` — MODIFIED: Added DryRun/Verbose/ProjectDir fields to App, ResolveProjectDir() method, persistent flags on root command
- `internal/cli/root_test.go` — CREATED: Tests for flag registration, defaults, and ResolveProjectDir behavior
- `internal/cli/run.go` — MODIFIED: Replaced os.Getwd() with app.ResolveProjectDir(), added WithDryRun/WithVerbose to pipeline
- `internal/cli/run_test.go` — MODIFIED: Added TestRunCommand_DryRunPropagated integration test
- `internal/cli/create_story.go` — MODIFIED: Replaced os.Getwd() with app.ResolveProjectDir(), added WithDryRun/WithVerbose to pipeline
- `internal/cli/validate_story.go` — MODIFIED: Replaced os.Getwd() with app.ResolveProjectDir(), added WithDryRun/WithVerbose to pipeline
- `internal/cli/preconditions.go` — MODIFIED: Replaced os.Getwd() with app.ResolveProjectDir()
- `internal/pipeline/steps.go` — MODIFIED: Added dry-run guard to StepValidate(), inlined stepSync() with dry-run guard and verbose output
- `internal/pipeline/steps_test.go` — MODIFIED: Added TestStepValidate_DryRun, TestStepSync_DryRun, TestStepSync_Verbose
- `internal/pipeline/printer_line_writer.go` — CREATED: Line-buffered `io.Writer` adapter for verbose bd output
- `internal/beads/executor.go` / `mock_executor.go` — MODIFIED: `Create(..., bdOut io.Writer)` tees subprocess stdout/stderr
- `internal/cli/{create_story,validate_story,epic,queue}_test.go` — MODIFIED: Dry-run propagation tests
