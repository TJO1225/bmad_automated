# Story 1.1: Strip Legacy Code and Rename Project

Status: done

## Story

As a developer,
I want the codebase cleaned of legacy commands and packages with the binary renamed to `story-factory`,
so that I have a clean, compiling foundation to build the new pipeline on.

## Acceptance Criteria

1. **Given** the forked codebase with legacy commands and packages
   **When** I run `just build`
   **Then** a `./story-factory` binary is produced with no compilation errors

2. **Given** the build succeeds
   **When** I inspect `go.mod`
   **Then** the Go module is renamed from `bmad-automate` to `story-factory`

3. **Given** the build succeeds
   **When** I inspect `internal/cli/`
   **Then** no legacy command files exist: `dev_story.go`, `code_review.go`, `git_commit.go`

4. **Given** the build succeeds
   **When** I inspect `internal/`
   **Then** no legacy packages exist: `state/`, `router/`, `lifecycle/`, `workflow/`

5. **Given** the binary is built
   **When** I run `./story-factory --help`
   **Then** only a root command is shown (no subcommands beyond Cobra defaults)
   **And** the command name displays as `story-factory`, not `bmad-automate`

## Tasks / Subtasks

> **Recommended execution order:** Delete files first (Tasks 2, 3, 7), then rename imports in surviving files only (Task 1). This avoids renaming imports in files that will be deleted.

- [x] Task 1: Rename Go module (AC: #2)
  - [x] 1.1 Change `go.mod` module line from `bmad-automate` to `story-factory`
  - [x] 1.2 Update ALL import paths across every `.go` file from `bmad-automate/internal/...` to `story-factory/internal/...`
  - [x] 1.3 Run `go mod tidy` to validate

- [x] Task 2: Delete legacy command files from `internal/cli/` (AC: #3)
  - [x] 2.1 Delete `internal/cli/dev_story.go`
  - [x] 2.2 Delete `internal/cli/code_review.go`
  - [x] 2.3 Delete `internal/cli/git_commit.go`
  - [x] 2.4 Delete `internal/cli/raw.go` (the `raw` command is not in the new architecture)

- [x] Task 3: Delete legacy packages from `internal/` (AC: #4)
  - [x] 3.1 Delete `internal/state/` entirely (resumability now via sprint-status.yaml re-read)
  - [x] 3.2 Delete `internal/router/` entirely (pipeline replaces routing)
  - [x] 3.3 Delete `internal/lifecycle/` entirely (pipeline replaces lifecycle)
  - [x] 3.4 Delete `internal/workflow/` entirely (pipeline replaces workflow)

- [x] Task 4: Strip legacy commands from root command registration (AC: #5)
  - [x] 4.1 In `internal/cli/root.go`, remove `newDevStoryCommand`, `newCodeReviewCommand`, `newGitCommitCommand`, `newRawCommand` from `rootCmd.AddCommand()`
  - [x] 4.2 Also remove `newCreateStoryCommand`, `newRunCommand`, `newQueueCommand`, `newEpicCommand` — these will be rebuilt from scratch in later stories
  - [x] 4.3 Remove the `StatusWriter` interface and field from `App` struct (Story Factory is read-only)
  - [x] 4.4 Remove the `WorkflowRunner` interface and field from `App` struct (pipeline replaces it)
  - [x] 4.5 Remove import for `workflow` package (the `status` import stays — Reader is retained). Also remove `context` import if no longer used after interface removal.
  - [x] 4.6 Update `NewApp()` to only wire retained dependencies (config, executor, printer, status reader)
  - [x] 4.7 Update `NewRootCommand()` to register no subcommands (empty for now)
  - [x] 4.8 Update `Use:` to `"story-factory"` and `Short:`/`Long:` descriptions

- [x] Task 5: Create entry point (AC: #1)
  - [x] 5.1 Create `cmd/story-factory/main.go` with `func main() { cli.Execute() }`
  - [x] 5.2 Delete `cmd/bmad-automate/` if it exists (it does NOT exist currently — the justfile references it but the directory is missing, so just create the new one)

- [x] Task 6: Update justfile (AC: #1)
  - [x] 6.1 Change `binary_name` from `"bmad-automate"` to `"story-factory"`
  - [x] 6.2 Change build/install paths from `./cmd/bmad-automate` to `./cmd/story-factory`

- [x] Task 7: Clean deleted command files that reference legacy types (AC: #3, #4)
  - [x] 7.1 In `internal/cli/epic.go`, `queue.go`, `run.go` — these files contain commands that will be rebuilt later; delete them now to avoid compile errors from references to removed types
  - [x] 7.2 Delete corresponding test files: `epic_test.go`, `queue_test.go`, `run_test.go`, `cli_test.go` that test removed functionality
  - [x] 7.3 Delete `internal/cli/create_story.go` (will be rebuilt in Story 2.2)
  - [x] 7.4 Keep `internal/cli/errors.go` and `errors_test.go` — they define `ExitError`/`IsExitError` used by root.go. No changes needed.
  - [x] 7.5 Delete `internal/cli/doc_test.go` if it references removed packages

- [x] Task 8: Clean config/workflows.yaml (AC: none — housekeeping)
  - [x] 8.1 Remove legacy workflow templates: `dev-story`, `code-review`, `git-commit`
  - [x] 8.2 Remove `full_cycle` section (pipeline replaces this)
  - [x] 8.3 Keep `create-story` prompt template (used by the pipeline in later stories)
  - [x] 8.4 Keep `claude` and `output` config sections

- [x] Task 9: Update CLAUDE.md (AC: none — housekeeping)
  - [x] 9.1 Update the package dependency diagram to reflect the new structure (remove workflow, router, lifecycle, state)
  - [x] 9.2 Update build commands if any changed
  - [x] 9.3 Update binary name references
  - [x] 9.4 Update "Claude CLI Integration" section: the current CLAUDE.md says executor passes `--dangerously-skip-permissions`; the new architecture uses `--enable-auto-mode` instead — update this reference

- [x] Task 10: Verify clean compilation (AC: #1, #5)
  - [x] 10.1 Run `go build ./cmd/story-factory` — must compile with zero errors
  - [x] 10.2 Run `./story-factory --help` — must show root command only
  - [x] 10.3 Run `go vet ./...` — must pass
  - [x] 10.4 Run `just test` — all remaining tests must pass (some tests will be deleted with their packages)

## Dev Notes

### What This Story Is NOT

- Do NOT rebuild any CLI commands (create-story, run, epic, queue) — those come in Epic 2
- Do NOT create new packages (pipeline, beads) — those come in later stories
- Do NOT rewrite the status package — that comes in Story 1.2
- Do NOT modify `internal/claude/`, `internal/config/`, or `internal/output/` beyond import path renames — those are retained packages

### Packages to RETAIN (only rename imports)

| Package | Why |
|---------|-----|
| `internal/claude/` | Core subprocess engine. `Executor`, `MockExecutor`, `Parser`, `Event` types are reused as-is. |
| `internal/config/` | Viper config loading. Will be simplified in a later story but compiles fine as-is. |
| `internal/output/` | Terminal formatting with lipgloss. `Printer` interface, `DefaultPrinter`. |
| `internal/status/` | Will be rewritten in Story 1.2, but leave it for now — the `Reader` is used by `NewApp()`. Remove only the `Writer` usage from `cli/root.go`. Leave `writer.go` and `writer_test.go` as dead code — they compile independently and Story 1.2 will delete/rewrite the entire package. Do NOT delete them now. |

### Import Path Rename Pattern

Every `.go` file in the project uses import paths like:
```go
import "bmad-automate/internal/claude"
```
These must ALL change to:
```go
import "story-factory/internal/claude"
```

**Approach:** Use `sed` or IDE refactor to do a global find-replace of `"bmad-automate/` with `"story-factory/` across all `.go` files. Then verify with `go build`.

### Current Codebase State

- **No `cmd/` directory exists** — the justfile references `./cmd/bmad-automate` but the directory was never created. You must create `cmd/story-factory/main.go` from scratch.
- **No `main.go` exists anywhere** — the project currently has no entry point.
- **`internal/cli/root.go`** is the de facto entry point via `cli.Execute()`.
- **`internal/cli/errors.go`** defines `ExitError` and `IsExitError` — keep this, it's used by `root.go`.

### Dependency Graph After This Story

```
cmd/story-factory/main.go
         |
         v
    internal/cli (root command only, no subcommands)
         |
         +---> internal/claude (retained, import paths updated)
         +---> internal/config (retained, import paths updated)
         +---> internal/output (retained, import paths updated)
         +---> internal/status (retained temporarily, Reader only)
```

### config/workflows.yaml Cleanup

The current `config/workflows.yaml` contains:
- `workflows.create-story` — **KEEP** (used by pipeline in later stories)
- `workflows.dev-story` — **DELETE** (legacy command removed)
- `workflows.code-review` — **DELETE** (legacy command removed)
- `workflows.git-commit` — **DELETE** (legacy command removed)
- `full_cycle` — **DELETE** (pipeline replaces the full-cycle concept)
- `claude` section — **KEEP** (executor config)
- `output` section — **KEEP** (output config)

### Project Structure Notes

- The project root is `/home/tjo/dev_wsl/dev-tools/bmad_automated/` but after rename the Go module will be `story-factory` (the directory name doesn't need to match the module name in Go)
- The justfile binary output changes from `bmad-automate` to `story-factory`

### Testing Strategy

After deletion, the remaining test surface is:
- `internal/claude/*_test.go` — should pass after import rename
- `internal/config/*_test.go` — should pass after import rename
- `internal/output/*_test.go` — should pass after import rename
- `internal/status/*_test.go` — should pass after import rename (Reader tests remain)
- `internal/cli/errors_test.go` — keep if it only tests `ExitError`/`IsExitError`

Run `just test` to verify all remaining tests pass after the strip.

### References

Source documents (load for full context if needed):
- `_bmad-output/planning-artifacts/architecture.md` — Package Disposition Plan, Project Structure, Implementation Handoff
- `_bmad-output/planning-artifacts/epics.md` — Story 1.1 acceptance criteria and epic context
- `_bmad-output/planning-artifacts/prd.md` — Additional Requirements (brownfield fork section)

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

No debug issues encountered. All tasks completed cleanly in one pass.

### Completion Notes List

- Followed recommended execution order: deletions first (Tasks 2, 3, 7), then rename (Task 1), then remaining tasks
- Deleted 4 legacy CLI command files: dev_story.go, code_review.go, git_commit.go, raw.go
- Deleted 4 legacy packages: state/, router/, lifecycle/, workflow/
- Deleted 5 additional CLI files: epic.go, queue.go, run.go, create_story.go and associated tests
- Deleted doc_test.go (referenced removed types extensively)
- Renamed Go module from bmad-automate to story-factory in go.mod and all import paths
- Rewrote root.go: removed WorkflowRunner and StatusWriter interfaces, stripped all command registrations, updated App struct and NewApp
- Created cmd/story-factory/main.go entry point
- Updated justfile binary name and build paths
- Cleaned workflows.yaml: removed dev-story, code-review, git-commit templates and full_cycle section
- Updated CLAUDE.md: new package diagram, binary name, --enable-auto-mode reference
- `go mod tidy` promoted gopkg.in/yaml.v3 to direct dependency (used by status package)
- Retained errors.go and errors_test.go (define ExitError/IsExitError used by root.go)
- Retained status package with writer.go/writer_test.go as dead code per Dev Notes (Story 1.2 will handle)
- All 5 test packages pass, go vet clean, go build clean

### File List

**Created:**
- cmd/story-factory/main.go

**Modified:**
- go.mod (module rename + go mod tidy)
- go.sum (updated by go mod tidy)
- justfile (binary name and build paths)
- config/workflows.yaml (removed legacy templates and full_cycle)
- CLAUDE.md (updated package diagram, binary name, CLI integration)
- internal/cli/root.go (stripped legacy interfaces, commands, updated App struct)
- internal/claude/doc_test.go (import path rename)
- internal/config/doc_test.go (import path rename)
- internal/output/doc_test.go (import path rename)
- internal/status/doc_test.go (import path rename)

**Deleted:**
- internal/cli/dev_story.go
- internal/cli/code_review.go
- internal/cli/git_commit.go
- internal/cli/raw.go
- internal/cli/epic.go
- internal/cli/queue.go
- internal/cli/run.go
- internal/cli/create_story.go
- internal/cli/cli_test.go
- internal/cli/epic_test.go
- internal/cli/queue_test.go
- internal/cli/run_test.go
- internal/cli/doc_test.go
- internal/state/ (entire package)
- internal/router/ (entire package)
- internal/lifecycle/ (entire package)
- internal/workflow/ (entire package)

### Review Findings

- [x] [Review][Decision] CLAUDE.md says `--enable-auto-mode` but `internal/claude/client.go` still passes `--dangerously-skip-permissions` — RESOLVED: updated client.go to use `--enable-auto-mode`
- [x] [Review][Patch] `.golangci.yml` `local-prefixes` still set to `bmad-automate` [.golangci.yml:32] — FIXED
- [x] [Review][Patch] `.gitignore` missing `story-factory` binary; old `bmad-automate` entry is stale [.gitignore:2] — FIXED
- [x] [Review][Patch] `internal/config/types.go` package doc comment still references "bmad-automate" [internal/config/types.go:1] — FIXED
- [x] [Review][Defer] `DefaultConfig()` defines 3 deleted workflows + `FullCycle` — deferred, spec says don't modify config package
- [x] [Review][Defer] `internal/status/types.go` godoc references deleted `internal/router` — deferred, Story 1.2 rewrites status package
- [x] [Review][Defer] `CycleHeader()` in `output/printer.go` hardcodes deleted pipeline — deferred, spec says don't modify output package
- [x] [Review][Defer] `README.md` / `CONTRIBUTING.md` use old project name throughout — deferred, not in story scope
- [x] [Review][Defer] `docs/` directory references old project name and deleted commands — deferred, not in story scope
- [x] [Review][Defer] `os.Stderr.WriteString` error return ignored in `root.go` — deferred, pre-existing

### Change Log

- 2026-04-06: Story 1.1 implemented — stripped legacy code, renamed project from bmad-automate to story-factory, created clean compiling foundation
- 2026-04-06: Code review completed — 1 decision-needed, 3 patches, 6 deferred, 9 dismissed
