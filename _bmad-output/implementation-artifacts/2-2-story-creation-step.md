# Story 2.2: Story Creation Step

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an operator,
I want to create a story file from a backlog entry by invoking a single command,
so that BMAD generates the complete story specification without manual Claude CLI invocation.

## Acceptance Criteria

1. **Given** a story key `1-2-database-schema` with status `backlog` in sprint-status.yaml
   **When** the operator runs `story-factory create-story 1-2-database-schema`
   **Then** a `claude -p` subprocess is spawned with `--enable-auto-mode --output-format stream-json`
   **And** the prompt includes the story key
   **And** the working directory is set to the project directory

2. **Given** the Claude subprocess completes successfully
   **When** the step verifies post-conditions
   **Then** the story file exists at the path resolved from `story_location`
   **And** re-reading `sprint-status.yaml` shows the story status changed from `backlog`
   **And** a `StepResult` with `Success: true` is returned

3. **Given** the story file does not exist after Claude completes
   **When** the step verifies post-conditions
   **Then** a `StepResult` with `Success: false` and a clear `Reason` is returned

4. **Given** a Claude subprocess exceeds the configured timeout
   **When** the timeout fires
   **Then** the subprocess receives SIGTERM, then SIGKILL after 5 seconds if still alive
   **And** no orphaned processes remain
   **And** a `StepResult` with `Success: false` and reason "timed out" is returned

## Tasks / Subtasks

- [x] Task 1: Add graceful shutdown support to `claude.DefaultExecutor` (AC: #4)
  - [x] Add `GracePeriod time.Duration` field to `ExecutorConfig` — when > 0, sets `cmd.Cancel` to send SIGTERM and `cmd.WaitDelay` to GracePeriod
  - [x] Apply `cmd.Cancel` and `cmd.WaitDelay` in both `Execute()` and `ExecuteWithResult()` methods after `cmd.Start()`... actually before Start, when building the cmd
  - [x] Default GracePeriod: 0 (preserves existing behavior — SIGKILL on cancel)
  - [x] Update `NewApp()` in `cli/root.go` to set `GracePeriod: 5 * time.Second` when constructing the executor
  - [x] Add test verifying that GracePeriod=0 still works (existing tests should pass unchanged)

- [x] Task 2: Define `pipeline.Printer` interface (AC: #2, #3)
  - [x] Create `internal/pipeline/printer.go`
  - [x] Define `Printer` interface with methods needed by pipeline steps:
    ```go
    type Printer interface {
        Text(message string)
        StepStart(step, total int, name string)
        StepEnd(duration time.Duration, success bool)
    }
    ```
  - [x] `output.DefaultPrinter` already satisfies this interface — no changes to `output/` needed
  - [x] DO NOT import `internal/output/` from `internal/pipeline/` — the interface is defined here, implemented there

- [x] Task 3: Create `Pipeline` struct (AC: all)
  - [x] Create `internal/pipeline/pipeline.go`
  - [x] Define minimal `Pipeline` struct:
    ```go
    type Pipeline struct {
        Claude     claude.Executor
        Status     *status.Reader
        Printer    Printer
        Cfg        *config.Config
        ProjectDir string
        DryRun     bool
        Verbose    bool
    }
    ```
  - [x] Add `NewPipeline()` constructor that validates required dependencies
  - [x] Export fields (uppercase) so `cli/` can construct it directly, OR use a `PipelineConfig` options struct — choose the simplest approach
  - [x] Pipeline does NOT hold a `beads.Executor` yet — that's Story 2.4

- [x] Task 4: Implement `stepCreate()` method (AC: #1, #2, #3, #4)
  - [x] Create `internal/pipeline/steps.go`
  - [x] Implement `func (p *Pipeline) StepCreate(ctx context.Context, key string) StepResult`:
    1. Record start time
    2. Expand prompt: `p.Cfg.GetPrompt("create-story", key)` — returns the expanded template from `config/workflows.yaml`
    3. Create timeout context: `context.WithTimeout(ctx, DefaultTimeout)` where `DefaultTimeout = 5 * time.Minute`
    4. Call `p.Claude.ExecuteWithResult(timeoutCtx, prompt, handler)` where handler forwards Text events to `p.Printer.Text()` if `p.Verbose`
    5. Check Claude exit code — non-zero means failure
    6. **Post-condition 1 — file exists:** Call `p.Status.ResolveStoryLocation(p.ProjectDir)` to get the story directory, then `os.Stat(filepath.Join(storyDir, key+".md"))` to verify the file was created
    7. **Post-condition 2 — status changed:** Create a fresh `status.NewReader(p.ProjectDir)` and call `StoryByKey(key)` — verify status is no longer `backlog`
    8. Return `StepResult{Name: "create", Success: true/false, Reason: ..., Duration: elapsed}`
  - [x] If context deadline exceeded (timeout), return `StepResult{Success: false, Reason: "create story " + key + ": timed out"}`
  - [x] If Claude returns non-zero exit code, return `StepResult{Success: false, Reason: "create story " + key + ": claude exited with code N"}`
  - [x] If file missing after success, return `StepResult{Success: false, Reason: "create story " + key + ": story file not created at <path>"}`
  - [x] If status unchanged, return `StepResult{Success: false, Reason: "create story " + key + ": sprint status not updated"}`
  - [x] Include story key in all failure reasons per architecture rule
  - [x] Define `DefaultTimeout = 5 * time.Minute` constant in this file (NFR9)

- [x] Task 5: Handle dry-run mode (AC: #1)
  - [x] In `StepCreate()`, if `p.DryRun` is true, skip subprocess invocation
  - [x] Return `StepResult{Name: "create", Success: true, Reason: "dry-run: would create story " + key}`
  - [x] Print planned operation via `p.Printer.Text()`

- [x] Task 6: Create `create-story` CLI command (AC: #1, #2, #3)
  - [x] Create `internal/cli/create_story.go`
  - [x] Define Cobra command:
    ```go
    func NewCreateStoryCommand(app *App) *cobra.Command {
        return &cobra.Command{
            Use:   "create-story <story-key>",
            Short: "Create a story file from a backlog entry",
            Args:  cobra.ExactArgs(1),
            RunE: func(cmd *cobra.Command, args []string) error {
                // 1. Run preconditions
                // 2. Construct Pipeline
                // 3. Call StepCreate(ctx, args[0])
                // 4. Handle StepResult
            },
        }
    }
    ```
  - [x] Call `app.RunPreconditions()` first — return `ExitError(2)` on failure
  - [x] Resolve `projectDir` from CWD (same as RunPreconditions does)
  - [x] Construct `Pipeline` with `app.Executor`, `status.NewReader(projectDir)`, `app.Printer` (cast to `pipeline.Printer`), `app.Config`, `projectDir`
  - [x] Call `pipeline.StepCreate(cmd.Context(), storyKey)`
  - [x] If `StepResult.Success` is false, print failure reason via `app.Printer.Text()` and return `NewExitError(1)`
  - [x] If success, print confirmation via `app.Printer.Text()`
  - [x] Register command in `NewRootCommand()` via `rootCmd.AddCommand(NewCreateStoryCommand(app))`

- [x] Task 7: Create `internal/pipeline/steps_test.go` — step-level unit tests (AC: #1, #2, #3, #4)
  - [x] Test `StepCreate` with successful Claude run:
    - Create `t.TempDir()` with valid `sprint-status.yaml` fixture (story in `backlog`)
    - Create story file at expected location (simulating BMAD's creation)
    - Update fixture YAML to show status changed (or mock the re-read)
    - Use `MockExecutor` with `ExitCode: 0`
    - Assert `StepResult.Success == true` and `StepResult.Name == "create"`
  - [x] Test `StepCreate` with file-not-created post-condition failure:
    - Create fixture YAML but do NOT create story file
    - Assert `StepResult.Success == false` and `Reason` mentions "not created"
  - [x] Test `StepCreate` with Claude non-zero exit:
    - Use `MockExecutor` with `ExitCode: 1`
    - Assert `StepResult.Success == false` and `Reason` mentions exit code
  - [x] Test `StepCreate` with Claude error (process fails to start):
    - Use `MockExecutor` with `Error: errors.New("binary not found")`
    - Assert error is returned (infrastructure failure, not StepResult)
  - [x] Test `StepCreate` with context timeout:
    - Pass already-cancelled context
    - Assert `StepResult.Success == false` and `Reason` mentions "timed out"
  - [x] Test `StepCreate` in dry-run mode:
    - Set `p.DryRun = true`
    - Assert `StepResult.Success == true` and no executor calls made
    - Verify `MockExecutor.RecordedPrompts` is empty
  - [x] Test prompt expansion:
    - Verify `MockExecutor.RecordedPrompts[0]` contains the story key
  - [x] Test verbose mode forwards Claude events to Printer:
    - Use MockExecutor with text events
    - Capture Printer output and verify Text() was called
  - [x] Use table-driven tests with `testify/assert` and `testify/require`

- [x] Task 8: Create `internal/cli/create_story_test.go` — CLI integration tests (AC: #1)
  - [x] Test command registration — verify `create-story` is a registered subcommand
  - [x] Test missing argument — verify error when no story key provided
  - [x] Test extra arguments — verify error when too many args
  - [x] Use `output.NewPrinterWithWriter(&buf)` to capture output

- [x] Task 9: Verify `just check` passes (fmt + vet + test)

## Dev Notes

### What This Story IS

Create the story creation pipeline step and its CLI command. This includes:
1. Graceful subprocess shutdown (SIGTERM -> 5s -> SIGKILL) in the Claude executor
2. The `pipeline.Printer` interface (defined in pipeline/, implemented by output/)
3. A minimal `Pipeline` struct that holds dependencies for step execution
4. The `StepCreate()` method with post-condition verification
5. The `create-story` Cobra subcommand
6. Tests for all the above

### What This Story is NOT

- Do NOT implement `stepValidate` or `stepSync` — those are Stories 2.3 and 2.4
- Do NOT implement `Pipeline.Run()`, `RunEpic()`, or `RunQueue()` — that's Story 2.5
- Do NOT create `internal/beads/` — that's Story 2.4
- Do NOT add `--project-dir`, `--verbose`, or `--dry-run` as CLI flags yet — those come in Story 3.3. For now, `projectDir` comes from CWD, `verbose` defaults to false, `dryRun` defaults to false. The Pipeline struct has fields for them, but the CLI command doesn't expose them as flags.
- Do NOT implement retry logic — that's Story 2.5's `Run()` method
- Do NOT modify `internal/status/` or `internal/config/` (they already have everything needed)

### Architecture Constraints

**Package dependency rules (must follow):**
```
cli/ → pipeline/ (StepCreate, Pipeline, Printer, StepResult)
cli/ → output/   (DefaultPrinter — satisfies pipeline.Printer)
cli/ → claude/   (Executor, ExecutorConfig)
cli/ → config/   (Config, GetPrompt)
cli/ → status/   (NewReader)
pipeline/ → claude/ (Executor interface)
pipeline/ → status/ (Reader, Entry, Status constants)
pipeline/ → config/ (Config, GetPrompt)
pipeline/ does NOT import output/
```

**Printer interface lives in `pipeline/`, not `output/`:**
The `pipeline.Printer` interface is defined in `internal/pipeline/printer.go`. The `output.DefaultPrinter` already has the required methods (`Text`, `StepStart`, `StepEnd`) so it implicitly satisfies `pipeline.Printer`. The CLI layer casts it:

```go
p := &pipeline.Pipeline{
    Printer: app.Printer, // output.DefaultPrinter satisfies pipeline.Printer
}
```

This works because Go interfaces are satisfied implicitly. No adapter or wrapper needed.

**Error representation (critical):**
- `StepResult` for operational outcomes: timeout, file not created, status unchanged, Claude non-zero exit
- `error` return for infrastructure failures: binary not found, filesystem unreadable, config error
- Steps return `(StepResult, error)` — StepResult for operational outcomes, error for infrastructure failures
- Include story key in all failure reasons: `"create story " + key + ": <reason>"`

**Subprocess invocation:**
- Claude CLI command: `claude -p "<prompt>" --output-format stream-json --enable-auto-mode`
- The executor already adds `--enable-auto-mode` and `--output-format stream-json` — do NOT add them again
- The executor already uses `exec.CommandContext(ctx, ...)` for cancellation support
- Working directory: the executor currently does NOT set the working directory. For story 2.2, the CLI command should `os.Chdir()` or the executor needs a `WorkingDir` config field. **Decision: Add `WorkingDir string` to `ExecutorConfig` and set `cmd.Dir` in Execute/ExecuteWithResult.** This is needed because the architecture says "Working directory set explicitly on all subprocesses via --project-dir value."

### Prompt Template

The prompt is expanded via `config.GetPrompt("create-story", storyKey)`. The template in `config/workflows.yaml` is:
```
/bmad:bmm:workflows:create-story - Create story: {{.StoryKey}}. Do not ask questions.
```

So for key `1-2-database-schema`, the prompt becomes:
```
/bmad:bmm:workflows:create-story - Create story: 1-2-database-schema. Do not ask questions.
```

The `GetPrompt` method is already implemented in `internal/config/config.go:120` and uses Go's `text/template` with `PromptData{StoryKey: storyKey}`.

### Post-Condition Verification Details

**File existence check (FR8):**
1. Get story directory: `reader.ResolveStoryLocation(projectDir)` — resolves `{project-root}/_bmad-output/implementation-artifacts` with the actual project dir
2. Build expected path: `filepath.Join(storyDir, key+".md")` — e.g., `/home/tom/project/_bmad-output/implementation-artifacts/1-2-database-schema.md`
3. `os.Stat(expectedPath)` — if err, file was not created

**Status change check (FR9):**
1. Create fresh Reader: `status.NewReader(projectDir)` — MUST create fresh, do not reuse the Pipeline's reader, to guarantee a fresh disk read (NFR4)
2. Call `reader.StoryByKey(key)` to get current status
3. Verify `entry.Status != status.StatusBacklog` — BMAD should have changed it to `ready-for-dev`
4. If still `backlog`, the creation succeeded in Claude's view but BMAD didn't update the status — this is a failure

### Executor Modification: Working Directory

The `DefaultExecutor` currently does NOT set `cmd.Dir`, so Claude runs in the process's CWD. The architecture requires explicit working directory:

> Working directory set explicitly on all subprocesses via --project-dir value

**Add `WorkingDir string` to `ExecutorConfig`.** When non-empty, set `cmd.Dir = config.WorkingDir` before `cmd.Start()`. Apply to both `Execute()` and `ExecuteWithResult()`.

Update `NewApp()` in `cli/root.go` to NOT set WorkingDir yet (it defaults to CWD). The `create-story` command resolves the project dir and passes it to Pipeline, which sets it on the executor.

**Wait — the executor is shared across the App.** We can't set WorkingDir per-invocation on a shared executor. Two options:
1. Make WorkingDir a parameter to Execute/ExecuteWithResult instead of a config field
2. Create the executor per-command (not shared)

**Recommended: Option 1** — Add an optional `WorkingDir` field to a new `ExecuteOptions` struct or add a `ExecuteInDir(ctx, prompt, dir, handler)` method. However, this changes the `Executor` interface, which is a bigger change.

**Simpler: Option 2** — The Pipeline creates its own executor with the right WorkingDir. The App provides the executor config (binary path, output format, etc.) and the Pipeline constructs a `claude.NewExecutor(...)` with the right WorkingDir. This means Pipeline depends on `claude.ExecutorConfig` too, but avoids interface changes.

**Simplest: Just set cmd.Dir in the existing methods.** Actually, the existing executor is fine — for now, the CLI command can just set the process CWD before calling the step. Or better: add a `Dir` field to Pipeline that stepCreate passes as context value... No, that's weird.

**FINAL DECISION: Add `WorkingDir string` to `ExecutorConfig` and set `cmd.Dir` in Execute/ExecuteWithResult.** The Pipeline constructs its own executor with WorkingDir set to projectDir. The App's executor (used for other things) keeps the default. This is the cleanest approach and follows the architecture.

But this means the Executor interface's Execute method signature doesn't change. The WorkingDir is a construction-time config, not per-invocation. The Pipeline creates a new executor for its project dir:

```go
// In create-story command:
executor := claude.NewExecutor(claude.ExecutorConfig{
    BinaryPath:   app.Config.Claude.BinaryPath,
    OutputFormat: app.Config.Claude.OutputFormat,
    WorkingDir:   projectDir,
    GracePeriod:  5 * time.Second,
})

p := &pipeline.Pipeline{
    Claude:     executor,
    Status:     status.NewReader(projectDir),
    Printer:    app.Printer,
    Cfg:        app.Config,
    ProjectDir: projectDir,
}
```

This means we don't need to change the Executor interface, just the ExecutorConfig and DefaultExecutor implementation.

### Testing Strategy

**`steps_test.go` in `internal/pipeline/`:**
- Create `t.TempDir()` for each test with controlled filesystem
- Write `sprint-status.yaml` fixture files to the temp dir at `_bmad-output/implementation-artifacts/sprint-status.yaml`
- For "file created" tests: create the story file in the temp dir before post-condition check runs
- For "status changed" tests: write the fixture YAML with updated status

**Key challenge:** The MockExecutor doesn't actually modify files or update sprint-status.yaml. So post-condition checks will find:
- No story file (unless the test creates it)
- No status change (unless the test writes an updated YAML)

**Solution:** The test setup must simulate BMAD's side effects:
1. Write initial sprint-status.yaml with story in `backlog`
2. Configure MockExecutor with a callback that creates the story file and updates the YAML
3. OR: Split the test — test the executor call separately from post-condition checks

**Recommended approach:** Use MockExecutor's handler to simulate side effects. Since `ExecuteWithResult` is synchronous and the handler is called for each event, you could use the last event as a trigger to create files. But MockExecutor doesn't support callbacks beyond the event handler...

**Practical approach:**
- For success tests: pre-create the story file AND write the updated YAML before constructing the Pipeline. Then call StepCreate — the executor runs (mock), and post-condition checks find the file and updated status.
- For failure tests: don't pre-create the file, so post-condition checks fail.
- This works because MockExecutor is instantaneous — there's no race between "Claude runs" and "file appears."

But wait — StepCreate creates a fresh Reader for the status check (to ensure fresh disk read). So the fixture YAML needs to already show the changed status. This simulates BMAD having updated the file during the Claude run.

Test fixture approach:
```
temp_dir/
  _bmad-output/implementation-artifacts/
    sprint-status.yaml   # With story status = "ready-for-dev" (simulating post-BMAD state)
    1-2-database-schema.md  # Story file (simulating BMAD creation)
```

For the "status not changed" test, the fixture YAML keeps the story in `backlog`.
For the "file not created" test, omit the story file.

**`create_story_test.go` in `internal/cli/`:**
- Test command registration on the root command
- Test argument validation (0 args → error, 2+ args → error)
- Use `output.NewPrinterWithWriter(&buf)` for output capture
- NOTE: Full integration test (with mock executor) may be complex for this layer — keep CLI tests thin, lean on pipeline/ tests for logic coverage

### Current Code State (from Story 2.1)

**New in `internal/pipeline/`:**
- `errors.go` — `ErrPreconditionFailed`, `PreconditionError` with `Check`/`Detail` fields
- `results.go` — `StepResult`, `StoryResult`, `BatchResult` structs (exact fields per architecture)
- `preconditions.go` — `CheckBdCLI()`, `CheckSprintStatus()`, `CheckBMADAgents()`, `CheckAll()`

**In `internal/cli/`:**
- `root.go` — `App` struct with `Config`, `Executor`, `Printer`, `StatusReader` fields. `NewApp()`, `NewRootCommand()`.
- `errors.go` — `ExitError`, `NewExitError()`, `IsExitError()`
- `preconditions.go` — `App.RunPreconditions()` calls `pipeline.CheckAll()`, exits with code 2

**In `internal/claude/`:**
- `client.go` — `Executor` interface, `DefaultExecutor`, `MockExecutor`. `Execute()` and `ExecuteWithResult()` methods.
- `ExecutorConfig` — `BinaryPath`, `OutputFormat`, `Parser`, `StderrHandler`

**In `internal/status/`:**
- `reader.go` — `Reader` with `Read()`, `BacklogStories()`, `StoriesForEpic()`, `StoryByKey()`, `ResolveStoryLocation()`. `DefaultStatusPath` constant.
- `types.go` — `Status`, `StatusBacklog`, `StatusReadyForDev`, etc. `Entry` struct with `Key`, `Status`, `Type`, `EpicNum`, `StoryNum`, `Slug`.

**In `internal/config/`:**
- `config.go` — `Config.GetPrompt(workflowName, storyKey)` expands template. `PromptData{StoryKey}`.
- `config/workflows.yaml` — prompt template for `create-story`

### Dependencies

- No new external dependencies
- stdlib: `context`, `os`, `path/filepath`, `time`, `fmt`, `errors`, `syscall` (for SIGTERM)
- Internal: `internal/claude`, `internal/status`, `internal/config` (from pipeline/); `internal/pipeline`, `internal/output` (from cli/)
- Test: `github.com/stretchr/testify` (already in go.mod)

### Project Structure Notes

New files created by this story:
```
internal/pipeline/
    printer.go         # pipeline.Printer interface
    pipeline.go        # Pipeline struct, NewPipeline()
    steps.go           # StepCreate() method
    steps_test.go      # Step unit tests
internal/cli/
    create_story.go    # create-story Cobra command
    create_story_test.go  # CLI smoke tests
```

Modified files:
```
internal/claude/client.go     # Add WorkingDir + GracePeriod to ExecutorConfig, set cmd.Dir + cmd.Cancel/WaitDelay
internal/cli/root.go          # Register create-story command in NewRootCommand(), update NewApp() with GracePeriod
```

No files deleted.

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 2, Story 2.2]
- [Source: _bmad-output/planning-artifacts/architecture.md - Pipeline Composition]
- [Source: _bmad-output/planning-artifacts/architecture.md - Subprocess Invocation Rules]
- [Source: _bmad-output/planning-artifacts/architecture.md - Error Representation]
- [Source: _bmad-output/planning-artifacts/architecture.md - Test Patterns]
- [Source: _bmad-output/planning-artifacts/architecture.md - Package Architecture]
- [Source: _bmad-output/planning-artifacts/prd.md - FR7, FR8, FR9, NFR4, NFR7, NFR9, NFR10]
- [Source: internal/claude/client.go - Executor interface, ExecutorConfig, DefaultExecutor]
- [Source: internal/status/reader.go - Reader, ResolveStoryLocation, DefaultStatusPath]
- [Source: internal/config/config.go - GetPrompt, PromptData]
- [Source: internal/cli/root.go - App struct, NewApp, NewRootCommand]
- [Source: internal/pipeline/results.go - StepResult struct]
- [Source: config/workflows.yaml - create-story prompt template]

### Previous Story Intelligence

From Story 2.1 (precondition-verification):
- **`pipeline/` package exists** with `errors.go`, `results.go`, `preconditions.go` — add new files alongside these
- **`StepResult` is defined** with exact fields from architecture: `Name`, `Success`, `Reason`, `Duration`, `ValidationLoops`, `BeadID` — use it directly, do not redefine
- **`App.RunPreconditions()` pattern** — call in create-story command before any processing. Uses `os.Getwd()` for project dir.
- **Error wrapping pattern** — `PreconditionError` wraps `ErrPreconditionFailed` via `Unwrap()`. Follow same pattern for any new error types.
- **testify conventions** — `assert` for soft checks, `require` for fatal preconditions. Table-driven tests.
- **`ExitError` with code 2** — precondition failures use exit code 2, general failures use exit code 1.
- **9 pipeline tests + 4 CLI tests** passed in story 2.1 — all must continue passing.

### Git Intelligence

Recent commits:
- `12c80ae` — refactor: rename project from bmad-automate to story-factory (module renamed, all imports updated)
- Module name is `story-factory` — all import paths use `story-factory/internal/...`
- `just check` is the standard verification command (fmt + vet + test)
- Go version: 1.25.5 (supports `cmd.Cancel` and `cmd.WaitDelay` for graceful subprocess shutdown)

### Review Findings

- [x] [Review][Patch] Distinguish infrastructure/read errors from “status not updated” in StepCreate post-check — When `StoryByKey` fails because the key is missing (`ErrStoryNotFound`) or `Read()` fails (I/O, parse), the code returns the same operational `StepResult` reason as “BMAD did not update status”, which violates clear failure reasons (AC #2–#3 intent) and hampers debugging. Prefer returning `(StepResult{}, err)` for read/parse failures, and a distinct `Reason` for unknown keys. [internal/pipeline/steps.go:161-170] — fixed 2026-04-06

- [x] [Review][Patch] “Timed out” reason on any context completion — After Claude returns an error, any non-nil `timeoutCtx.Err()` maps to `Reason: "... timed out"`, including parent `context.Canceled`. Users who interrupt the CLI see a timeout message instead of canceled. Use `errors.Is` on `DeadlineExceeded` vs `Canceled` (and optionally other errors) to match AC #4 wording. [internal/pipeline/steps.go:125-133] — fixed 2026-04-06

- [x] [Review][Patch] Nil `status` reader panics — `StepCreate` calls `p.status.ResolveStoryLocation` without a guard. Only the CLI wires `WithStatus` today; other callers get a nil pointer panic. Validate `p.status != nil` and return a wrapped infrastructure error. [internal/pipeline/steps.go:146-149] — fixed 2026-04-06

- [x] [Review][Patch] Context-timeout test is weak/misleading — `TestStepCreate_ContextTimeout` cancels the parent context and uses `MockExecutor{Error: context.DeadlineExceeded}`; the name implies deadline testing, and the body skips strong assertions when `err != nil`. Tighten with `require.NoError`, assert `Reason` contains `timed out`, and/or drive a real `context.WithDeadline`. [internal/pipeline/steps_test.go:333-350] — fixed 2026-04-06

- [x] [Review][Patch] Empty story key — `cobra.ExactArgs(1)` allows `""`, producing nonsense paths and prompts. Reject `strings.TrimSpace(args[0]) == ""` in the command or step. [internal/cli/create_story.go:21-22] — fixed 2026-04-06

- [x] [Review][Defer] No test exercises real subprocess SIGTERM → WaitDelay → SIGKILL — `GracePeriod` is only asserted on config in unit tests; AC #4 behavior is not integration-tested without a subprocess harness or test binary. [internal/claude/client.go:235-246] — deferred, pre-existing gap until a subprocess test strategy exists

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

### Completion Notes List

- Task 1: Added `GracePeriod` and `WorkingDir` fields to `ExecutorConfig`. `applyCommandConfig()` helper sets `cmd.Dir`, `cmd.Cancel` (SIGTERM), and `cmd.WaitDelay` before Start. Updated `NewApp()` to set 5s grace period.
- Task 2: Created `pipeline.Printer` interface in `printer.go` with `Text`, `StepStart`, `StepEnd` methods. `output.DefaultPrinter` satisfies it implicitly.
- Task 3: Extended existing `Pipeline` struct with `status`, `printer`, `dryRun`, `verbose` fields. Used functional options pattern (`WithStatus`, `WithPrinter`, `WithDryRun`, `WithVerbose`) to maintain backward compatibility with existing `NewPipeline` callers.
- Task 4: Implemented `StepCreate()` with prompt expansion, timeout context, Claude invocation, and two post-condition checks (file exists, status changed). Returns `(StepResult, error)` following the architecture's dual-return pattern.
- Task 5: Dry-run early return in `StepCreate()` — skips subprocess, prints via Printer, returns success with reason.
- Task 6: Created `create-story` Cobra command with precondition checks, per-invocation executor with WorkingDir, pipeline construction, and result handling. Registered in `NewRootCommand()`.
- Task 7: 8 StepCreate unit tests covering success, file-not-created, status-not-updated, non-zero exit, Claude error, context timeout, dry-run, prompt expansion, and verbose forwarding. All pass.
- Task 8: 3 CLI tests covering command registration, missing arg, and extra args. All pass.
- Task 9: `just check` passes (fmt + vet + test) — 231 tests across all packages.

### File List

New files:
- internal/pipeline/printer.go
- internal/cli/create_story.go
- internal/cli/create_story_test.go

Modified files:
- internal/claude/client.go (added WorkingDir, GracePeriod to ExecutorConfig, applyCommandConfig helper)
- internal/claude/client_test.go (added GracePeriod/WorkingDir assertions to TestNewExecutor)
- internal/cli/root.go (GracePeriod in NewApp, register create-story command)
- internal/pipeline/steps.go (added StepCreate method, Pipeline fields, option functions, DefaultTimeout constant)
- internal/pipeline/steps_test.go (added 8 StepCreate tests)

## Change Log

- 2026-04-06: Implemented story creation pipeline step with graceful shutdown, Printer interface, Pipeline extensions, StepCreate method, create-story CLI command, and comprehensive tests (9 tasks completed)
- 2026-04-06: Code review patches — StepCreate error taxonomy (timeout vs canceled, unknown key vs read errors), nil status guard, empty CLI key, stronger tests; StepValidate loop records `timeoutCtx.Err()` before `cancel()` to avoid misclassifying failures
