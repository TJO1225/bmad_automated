---
stepsCompleted:
  - step-01-validate-prerequisites
  - step-02-design-epics
  - step-03-create-stories
  - step-04-final-validation
inputDocuments:
  - _bmad-output/planning-artifacts/prd.md
  - _bmad-output/planning-artifacts/architecture.md
---

# Story Factory - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for Story Factory, decomposing the requirements from the PRD and Architecture into implementable stories.

## Requirements Inventory

### Functional Requirements

FR1: Operator can read and parse a BMAD v6 flat `development_status` YAML map from `sprint-status.yaml`
FR2: Operator can look up a specific story entry by its key and retrieve its current status
FR3: Operator can retrieve all stories with a given status (e.g., all `backlog` stories)
FR4: Operator can retrieve all stories belonging to a specific epic by epic number
FR5: System can classify each entry as epic (`epic-N`), story (`N-M-slug`), or retrospective (`epic-N-retrospective`)
FR6: System can resolve the `story_location` path template by replacing `{project-root}` with the actual project directory
FR7: Operator can invoke story creation for a single story by key, which spawns a `claude -p` subprocess with `--enable-auto-mode --output-format stream-json`
FR8: System can verify that a story file was created at the expected path after creation completes
FR9: System can re-read `sprint-status.yaml` after creation to confirm status changed from `backlog` to `ready-for-dev`
FR10: Operator can invoke story validation for a single story by key, using the same `claude -p` command (BMAD auto-detects validation mode when story file exists)
FR11: System can auto-accept all improvement suggestions during validation
FR12: System can re-validate after accepting suggestions, up to a maximum of 3 iterations
FR13: System can mark a story as `needs-review` if validation does not converge after 3 iterations
FR14: Operator can sync a validated story to Gastown Beads by invoking `bd create` with the story title and acceptance criteria
FR15: System can extract the story title from the first `# Story X.Y: Title` heading in the story markdown
FR16: System can extract acceptance criteria from the content between `## Acceptance Criteria` and the next `##` heading
FR17: System can parse the bead ID from `bd create` stdout
FR18: System can append a `<!-- bead:{bead-id} -->` tracking comment to the story file after successful sync
FR19: Operator can run the full pipeline (create -> validate -> sync) for a single story with the `run` command
FR20: Operator can run the full pipeline for all backlog stories in an epic with the `epic` command
FR21: Operator can run the full pipeline for all backlog stories across all epics with the `queue` command
FR22: System processes stories sequentially within an epic, in key order
FR23: System processes epics sequentially in numeric order (MVP)
FR24: System re-reads `sprint-status.yaml` between story operations to reflect BMAD's updates
FR25: System skips stories that are no longer in `backlog` status, enabling resumable batch runs
FR26: System retries a failed step once before marking the story as failed and continuing the batch
FR27: System treats the pipeline as a unit — failure at any step (create, validate, or sync) marks the entire story as failed
FR28: System can verify that `bd` CLI is available on PATH before processing
FR29: System can verify that `sprint-status.yaml` exists at the expected location before processing
FR30: System can verify that BMAD agent files are present before processing
FR31: System exits with a fatal error (exit code 2) if any precondition fails
FR32: Operator can run any batch command with `--dry-run` to see planned operations without executing
FR33: Operator can run any command with `--verbose` to stream Claude CLI output in real time
FR34: Operator can specify a non-CWD project directory with `--project-dir`
FR35: System displays a structured summary on batch completion showing: stories processed, validation iterations per story, bead IDs created, and failures with reasons
FR36: System exits with code 0 on full success, code 1 on partial failure, and code 2 on fatal precondition error

### NonFunctional Requirements

NFR1: System must be compatible with Claude CLI's `--output-format stream-json` streaming protocol and `--enable-auto-mode` flag
NFR2: System must parse BMAD v6 flat `development_status` YAML format; reject unrecognized formats with a clear error message rather than silently producing wrong results
NFR3: System must parse `bd create` stdout to extract bead IDs; fail gracefully with a clear error if `bd` output format is unrecognized
NFR4: System must treat `sprint-status.yaml` as an external mutable resource — always re-read from disk before each story operation, never cache stale state
NFR5: A failure in one story's pipeline must not affect processing of subsequent stories in a batch run
NFR6: System must never leave `sprint-status.yaml` in a corrupted state — since Story Factory only reads this file (BMAD writes it), this is enforced by design
NFR7: System must not leave orphaned Claude subprocesses on crash or timeout — subprocess cleanup must be handled on all exit paths
NFR8: After any failure, the system state must allow the same command to be re-run without manual cleanup (idempotent resumability)
NFR9: Claude subprocess timeout must be configurable, with a default sufficient for complex story generation (recommended: 5 minutes per invocation)
NFR10: Tool startup and precondition checks must complete in under 2 seconds — operator should see feedback immediately

### Additional Requirements

- Brownfield fork: strip legacy commands/packages (`dev-story`, `code-review`, `git-commit`, `state/`, `router/`, `lifecycle/`), rename binary from `bmad-automate` to `story-factory`, rename Go module
- Package consolidation: merge `lifecycle/`, `workflow/`, `router/`, `state/` into single `internal/pipeline/` package
- New `internal/beads/` package for `bd create` integration with `Executor` interface mirroring `claude.Executor` pattern
- New read-only `internal/status/` parser: `Reader` with `BacklogStories()`, `StoriesForEpic(n)`, `StoryByKey(key)` — remove existing `Writer`
- `Printer` interface must live in `internal/pipeline/` (not `output/`) to avoid circular dependency; `output/` implements it
- Result types (`StepResult`, `StoryResult`, `BatchResult`) defined in `internal/pipeline/results.go`
- Sentinel errors for expected conditions: `ErrStoryNotFound`, `ErrPreconditionFailed`, `ErrStoryComplete`
- Operational outcomes returned as `StepResult` (value type), infrastructure failures returned as `error`
- Validation loop uses mtime-based detection: compare story file modification timestamp before/after Claude invocation to determine if suggestions were applied
- Subprocess cleanup: SIGTERM -> 5s grace -> SIGKILL for Claude; 30s timeout for `bd create`
- Signal handling at CLI level: SIGINT/SIGTERM cancels root context, propagating cleanup to all in-flight subprocesses
- All subprocess invocations must use `context.Context` with configurable timeout
- Working directory set explicitly on all subprocesses via `--project-dir` value
- Claude prompt: `bmad-bmm-create-story <story-key>` (single template)
- Beads command: `bd create "<key>: <title>" --notes "<acs>"`
- Co-located tests (`<file>_test.go`), fixtures in `testdata/` per package, table-driven with `testify`
- All user-facing output through `Printer` — no `fmt.Println` outside `internal/output/`
- Implementation sequence: strip-and-compile -> status parser -> beads package -> pipeline package -> CLI commands -> polish

### UX Design Requirements

No UX Design document — this is a CLI tool with no graphical interface. UX requirements are not applicable.

### FR Coverage Map

FR1: Epic 1 — Parse BMAD v6 flat development_status YAML
FR2: Epic 1 — Look up story entry by key
FR3: Epic 1 — Retrieve all stories by status
FR4: Epic 1 — Retrieve stories by epic number
FR5: Epic 1 — Classify entries (epic/story/retrospective)
FR6: Epic 1 — Resolve story_location path template
FR7: Epic 2 — Invoke story creation via claude -p subprocess
FR8: Epic 2 — Verify story file created at expected path
FR9: Epic 2 — Re-read sprint-status.yaml to confirm status change
FR10: Epic 2 — Invoke story validation via claude -p
FR11: Epic 2 — Auto-accept improvement suggestions
FR12: Epic 2 — Re-validate up to 3 iterations
FR13: Epic 2 — Mark needs-review on validation exhaustion
FR14: Epic 2 — Sync validated story to Beads via bd create
FR15: Epic 2 — Extract story title from markdown heading
FR16: Epic 2 — Extract acceptance criteria from markdown
FR17: Epic 2 — Parse bead ID from bd create stdout
FR18: Epic 2 — Append bead tracking comment to story file
FR19: Epic 2 — Full pipeline (create->validate->sync) for single story
FR20: Epic 3 — Full pipeline for all backlog stories in an epic
FR21: Epic 3 — Full pipeline for all backlog stories across all epics
FR22: Epic 3 — Sequential story processing in key order
FR23: Epic 3 — Sequential epic processing in numeric order
FR24: Epic 3 — Re-read sprint-status.yaml between story operations
FR25: Epic 3 — Skip non-backlog stories (resumable runs)
FR26: Epic 2 — Retry failed step once before marking failed
FR27: Epic 2 — Pipeline-as-unit failure semantics
FR28: Epic 2 — Verify bd CLI on PATH
FR29: Epic 2 — Verify sprint-status.yaml exists
FR30: Epic 2 — Verify BMAD agent files present
FR31: Epic 2 — Exit code 2 on precondition failure
FR32: Epic 3 — Dry-run mode for batch commands
FR33: Epic 3 — Verbose mode for streaming Claude output
FR34: Epic 3 — --project-dir flag for non-CWD projects
FR35: Epic 3 — Structured summary on batch completion
FR36: Epic 3 — Exit code semantics (0/1/2)

## Epic List

### Epic 1: Foundation & Sprint Status Reading
Strip legacy code, rename binary/module to `story-factory`, build read-only sprint status parser, and establish project config. After this epic, the tool compiles as `story-factory` and can read, classify, and query entries from `sprint-status.yaml`.
**FRs covered:** FR1, FR2, FR3, FR4, FR5, FR6

### Epic 2: Single Story Processing Pipeline
Build the complete create->validate->sync pipeline for processing a single story, including precondition checks, Claude subprocess integration, validation loop with auto-accept, Beads sync with markdown parsing, retry-once semantics, and all four single-story CLI commands. After this epic, the operator can process any single backlog story through the full pipeline with `story-factory run <key>`.
**FRs covered:** FR7, FR8, FR9, FR10, FR11, FR12, FR13, FR14, FR15, FR16, FR17, FR18, FR19, FR26, FR27, FR28, FR29, FR30, FR31

### Epic 3: Batch Operations & Reporting
Build the `epic` and `queue` batch commands, sequential ordering, YAML re-read between operations, resumable run support, dry-run/verbose/project-dir flags, structured summary reporting, and exit code semantics. After this epic, the operator can process entire epics or the full backlog unattended and get a comprehensive completion report.
**FRs covered:** FR20, FR21, FR22, FR23, FR24, FR25, FR32, FR33, FR34, FR35, FR36

## Epic 1: Foundation & Sprint Status Reading

Strip legacy code, rename binary/module to `story-factory`, build read-only sprint status parser, and establish project config. After this epic, the tool compiles as `story-factory` and can read, classify, and query entries from `sprint-status.yaml`.

### Story 1.1: Strip Legacy Code and Rename Project

As a developer,
I want the codebase cleaned of legacy commands and packages with the binary renamed to `story-factory`,
So that I have a clean, compiling foundation to build the new pipeline on.

**Acceptance Criteria:**

**Given** the forked codebase with legacy commands and packages
**When** I run `just build`
**Then** a `./story-factory` binary is produced with no compilation errors
**And** the Go module is renamed to `story-factory`
**And** no legacy command files (`dev-story`, `code-review`, `git-commit`) exist in `internal/cli/`
**And** no legacy packages (`state/`, `router/`, `lifecycle/`, `workflow/`) exist in `internal/`
**And** `./story-factory --help` shows only a root command (no subcommands yet beyond any Cobra defaults)

### Story 1.2: Sprint Status YAML Parser

As an operator,
I want the tool to parse `sprint-status.yaml` and classify each entry,
So that the system understands which entries are epics, stories, or retrospectives.

**Acceptance Criteria:**

**Given** a valid `sprint-status.yaml` with BMAD v6 flat `development_status` format
**When** the Reader parses the file
**Then** all entries are loaded with their keys and status values
**And** entries with key pattern `epic-N` are classified as epic type
**And** entries with key pattern `N-M-slug` are classified as story type
**And** entries with key pattern `epic-N-retrospective` are classified as retrospective type

**Given** a YAML file with an unrecognized format (e.g., nested `sprints[].stories[]`)
**When** the Reader attempts to parse it
**Then** a clear error message is returned identifying the format as unrecognized
**And** no partial or incorrect data is produced

**Given** an empty `development_status` map
**When** the Reader parses the file
**Then** the Reader returns successfully with zero entries

### Story 1.3: Sprint Status Queries and Path Resolution

As an operator,
I want to look up stories by key, filter by status or epic number, and resolve story file paths,
So that the pipeline can determine which stories need processing and where their files live.

**Acceptance Criteria:**

**Given** a parsed sprint status with multiple entries
**When** I call `StoryByKey("1-2-database-schema")`
**Then** the matching entry is returned with its status
**And** an `ErrStoryNotFound` error is returned if the key doesn't exist

**Given** a parsed sprint status with stories in various statuses
**When** I call `StoriesByStatus("backlog")`
**Then** only entries with `backlog` status are returned
**And** entries are returned in key order

**Given** a parsed sprint status with stories across multiple epics
**When** I call `StoriesForEpic(2)`
**Then** only story entries belonging to epic 2 (keys matching `2-M-slug`) are returned
**And** entries are returned in key order

**Given** a story entry with `story_location: "{project-root}/_bmad-output/implementation-artifacts/story-1-2.md"`
**When** I call `ResolveStoryLocation` with project dir `/home/tom/projects/my-app`
**Then** the resolved path is `/home/tom/projects/my-app/_bmad-output/implementation-artifacts/story-1-2.md`

## Epic 2: Single Story Processing Pipeline

Build the complete create->validate->sync pipeline for processing a single story, including precondition checks, Claude subprocess integration, validation loop with auto-accept, Beads sync with markdown parsing, retry-once semantics, and all four single-story CLI commands. After this epic, the operator can process any single backlog story through the full pipeline with `story-factory run <key>`.

### Story 2.1: Precondition Verification

As an operator,
I want the tool to verify that all required dependencies are available before processing,
So that I get immediate, clear feedback if my environment isn't ready instead of a cryptic failure mid-pipeline.

**Acceptance Criteria:**

**Given** the `bd` CLI is not installed or not on PATH
**When** the operator runs any processing command
**Then** the tool prints a clear error message identifying `bd` as missing
**And** exits with code 2

**Given** `sprint-status.yaml` does not exist at the expected project location
**When** the operator runs any processing command
**Then** the tool prints a clear error message identifying the missing file and expected path
**And** exits with code 2

**Given** BMAD agent files are not present in the project
**When** the operator runs any processing command
**Then** the tool prints a clear error message identifying the missing agent files
**And** exits with code 2

**Given** all preconditions are met (`bd` on PATH, YAML exists, agent files present)
**When** the operator runs any processing command
**Then** precondition checks complete in under 2 seconds (NFR10)
**And** processing proceeds normally

### Story 2.2: Story Creation Step

As an operator,
I want to create a story file from a backlog entry by invoking a single command,
So that BMAD generates the complete story specification without manual Claude CLI invocation.

**Acceptance Criteria:**

**Given** a story key `1-2-database-schema` with status `backlog` in sprint-status.yaml
**When** the operator runs `story-factory create-story 1-2-database-schema`
**Then** a `claude -p` subprocess is spawned with `--enable-auto-mode --output-format stream-json`
**And** the prompt includes the story key
**And** the working directory is set to the project directory

**Given** the Claude subprocess completes successfully
**When** the step verifies post-conditions
**Then** the story file exists at the path resolved from `story_location`
**And** re-reading `sprint-status.yaml` shows the story status changed from `backlog`
**And** a `StepResult` with `Success: true` is returned

**Given** the story file does not exist after Claude completes
**When** the step verifies post-conditions
**Then** a `StepResult` with `Success: false` and a clear `Reason` is returned

**Given** a Claude subprocess exceeds the configured timeout
**When** the timeout fires
**Then** the subprocess receives SIGTERM, then SIGKILL after 5 seconds if still alive
**And** no orphaned processes remain
**And** a `StepResult` with `Success: false` and reason "timed out" is returned

### Story 2.3: Story Validation with Auto-Accept Loop

As an operator,
I want to validate a story with automatic suggestion acceptance up to 3 iterations,
So that story quality is ensured without manual review of each suggestion.

**Acceptance Criteria:**

**Given** a story with status `ready-for-dev` and an existing story file
**When** the operator runs `story-factory validate-story 1-2-database-schema`
**Then** a `claude -p` subprocess is spawned with `--enable-auto-mode`
**And** the working directory is set to the project directory

**Given** the story file mtime does not change after a validation invocation
**When** the step checks the mtime
**Then** validation is considered clean (converged)
**And** a `StepResult` with `Success: true` and `ValidationLoops: 1` is returned

**Given** the story file mtime changes after validation (suggestions were applied)
**When** the step detects the mtime change
**Then** validation is re-invoked automatically
**And** this continues up to a maximum of 3 total iterations

**Given** the story file mtime still changes on the 3rd iteration
**When** the maximum iteration count is reached
**Then** a `StepResult` with `Success: false` and reason `needs-review` is returned
**And** `ValidationLoops: 3` is recorded

**Given** validation converges on the 2nd iteration (mtime unchanged)
**When** the step completes
**Then** a `StepResult` with `Success: true` and `ValidationLoops: 2` is returned

### Story 2.4: Beads Synchronization

As an operator,
I want to sync a validated story to Gastown Beads with a single command,
So that a Bead is created and the story file is tagged with the bead ID for traceability.

**Acceptance Criteria:**

**Given** a validated story file with heading `# Story 1.2: Database Schema`
**When** the markdown parser extracts the title
**Then** the result is `Database Schema`

**Given** a story file with an `## Acceptance Criteria` section followed by another `##` heading
**When** the markdown parser extracts acceptance criteria
**Then** all content between the two headings is captured
**And** leading/trailing whitespace is trimmed

**Given** a validated story with extracted title and acceptance criteria
**When** the operator runs `story-factory sync-to-beads 1-2-database-schema`
**Then** `bd create "1-2-database-schema: Database Schema" --notes "<acs>"` is invoked
**And** the bead ID is parsed from stdout

**Given** `bd create` returns successfully with a bead ID
**When** the step completes post-processing
**Then** `<!-- bead:{bead-id} -->` is appended to the end of the story file
**And** a `StepResult` with `Success: true` and `BeadID` populated is returned

**Given** `bd create` fails or returns unrecognized output
**When** the step handles the failure
**Then** a `StepResult` with `Success: false` and a clear reason is returned
**And** the story file is not modified (no partial tracking comment)

### Story 2.5: Full Pipeline Composition & Run Command

As an operator,
I want to run the complete create->validate->sync pipeline for a single story with one command,
So that I can process a backlog story end-to-end without invoking each step manually.

**Acceptance Criteria:**

**Given** a story `1-2-database-schema` in `backlog` status with all preconditions met
**When** the operator runs `story-factory run 1-2-database-schema`
**Then** the tool executes create -> validate -> sync in sequence
**And** a `StoryResult` with `Success: true`, `ValidationLoops`, and `BeadID` is returned
**And** progress is displayed through the Printer

**Given** the create step fails on first attempt
**When** the pipeline retries
**Then** the create step is retried exactly once
**And** if the retry succeeds, the pipeline continues with validate and sync

**Given** the create step fails on both attempts (initial + retry)
**When** the pipeline handles the failure
**Then** a `StoryResult` with `Success: false`, `FailedAt: "create"`, and `Reason` is returned
**And** validate and sync steps are NOT attempted

**Given** the validate step fails after retry
**When** the pipeline handles the failure
**Then** a `StoryResult` with `Success: false` and `FailedAt: "validate"` is returned
**And** the sync step is NOT attempted

**Given** the story status is not `backlog`
**When** the operator runs `story-factory run 1-2-database-schema`
**Then** the pipeline returns a `StoryResult` with `Skipped: true`
**And** no subprocess is invoked

## Epic 3: Batch Operations & Reporting

Build the `epic` and `queue` batch commands, sequential ordering, YAML re-read between operations, resumable run support, dry-run/verbose/project-dir flags, structured summary reporting, and exit code semantics. After this epic, the operator can process entire epics or the full backlog unattended and get a comprehensive completion report.

### Story 3.1: Epic Batch Command

As an operator,
I want to process all backlog stories in an epic with a single command,
So that I can batch-process an entire epic unattended instead of running each story individually.

**Acceptance Criteria:**

**Given** Epic 1 has 3 backlog stories: `1-1-scaffold`, `1-2-schema`, `1-3-registration`
**When** the operator runs `story-factory epic 1`
**Then** stories are processed sequentially in key order (1-1, 1-2, 1-3)
**And** each story goes through the full create->validate->sync pipeline
**And** a `BatchResult` is returned with all 3 `StoryResult` entries

**Given** `sprint-status.yaml` is modified by BMAD after story `1-1` is processed
**When** the pipeline moves to story `1-2`
**Then** `sprint-status.yaml` is re-read from disk before processing `1-2`
**And** the fresh status is used for precondition checks

**Given** story `1-2` was already processed in a prior run (status is `ready-for-dev`)
**When** the batch encounters `1-2`
**Then** `1-2` is skipped with `Skipped: true` in its `StoryResult`
**And** the batch continues to `1-3`

**Given** story `1-2` fails after retry
**When** the batch handles the failure
**Then** `1-2` is recorded as failed in the `BatchResult`
**And** the batch continues processing `1-3` (no abort)

**Given** no stories in Epic 1 have `backlog` status
**When** the operator runs `story-factory epic 1`
**Then** the tool reports that no backlog stories were found for the epic
**And** exits with code 0

### Story 3.2: Queue Command & Cross-Epic Ordering

As an operator,
I want to process the entire backlog across all epics with a single command,
So that I can fire-and-forget the full sprint's story processing.

**Acceptance Criteria:**

**Given** a backlog with stories across Epic 1 (2 stories), Epic 2 (3 stories), and Epic 3 (1 story)
**When** the operator runs `story-factory queue`
**Then** epics are processed in numeric order: Epic 1, then Epic 2, then Epic 3
**And** within each epic, stories are processed in key order
**And** a single `BatchResult` contains all 6 `StoryResult` entries

**Given** Epic 1 stories are already processed (non-backlog) but Epic 2 has backlog stories
**When** the operator runs `story-factory queue`
**Then** Epic 1 stories are skipped
**And** Epic 2 stories are processed normally

**Given** no backlog stories exist anywhere
**When** the operator runs `story-factory queue`
**Then** the tool reports that no backlog stories were found
**And** exits with code 0

### Story 3.3: Dry-Run, Verbose, and Project-Dir Flags

As an operator,
I want to preview planned operations, stream real-time output, and target non-CWD projects,
So that I can verify batch plans before committing, monitor complex story processing, and work across multiple projects.

**Acceptance Criteria:**

**Given** Epic 1 has 3 backlog stories
**When** the operator runs `story-factory epic 1 --dry-run`
**Then** the tool reads `sprint-status.yaml` and lists planned operations per story
**And** each planned operation shows: story key, steps that would execute (create->validate->sync)
**And** no Claude or bd subprocess is invoked
**And** no files are created or modified

**Given** the operator runs with `--verbose`
**When** a Claude subprocess streams JSON events
**Then** text events are forwarded to the Printer and displayed in real time
**And** bd stdout is also displayed

**Given** the operator runs without `--verbose`
**When** a Claude subprocess streams JSON events
**Then** only progress indicators are displayed (not streaming content)

**Given** the operator runs `story-factory epic 1 --project-dir /home/tom/projects/other-app`
**When** the tool resolves paths and spawns subprocesses
**Then** `sprint-status.yaml` is read from `/home/tom/projects/other-app/`
**And** all subprocess working directories are set to `/home/tom/projects/other-app/`
**And** story file paths are resolved relative to that directory

### Story 3.4: Structured Summary & Exit Codes

As an operator,
I want a clear summary report after batch processing and meaningful exit codes,
So that I can quickly assess what landed, what failed, and script around the tool's outcomes.

**Acceptance Criteria:**

**Given** a `BatchResult` with 5 stories: 4 succeeded, 1 failed at validate
**When** the Printer displays the summary
**Then** a structured table shows each story with its status
**And** successful stories show checkmark, validation loops, and bead ID
**And** the failed story shows X, `FailedAt: "validate"`, and the failure reason
**And** totals show: `4 created, 4 validated, 4 synced, 1 failed`

**Given** a queue `BatchResult` spanning multiple epics
**When** the Printer displays the summary
**Then** results are grouped by epic with epic headers
**And** per-epic subtotals are shown

**Given** all stories in a batch succeeded
**When** the command exits
**Then** the exit code is `0`

**Given** at least one story failed but others succeeded
**When** the command exits
**Then** the exit code is `1`

**Given** a precondition check fails before any processing
**When** the command exits
**Then** the exit code is `2`

**Given** a `BatchResult` with skipped stories (already processed in prior run)
**When** the Printer displays the summary
**Then** skipped stories are shown with a distinct indicator (not success, not failure)
**And** the skip count is included in totals
