---
stepsCompleted:
  - step-01-init
  - step-02-discovery
  - step-02b-vision
  - step-02c-executive-summary
  - step-03-success
  - step-04-journeys
  - step-05-domain
  - step-06-innovation
  - step-07-project-type
  - step-08-scoping
  - step-09-functional
  - step-10-nonfunctional
  - step-11-polish
  - step-12-complete
inputDocuments:
  - _bmad-output/planning-artifacts/product-brief-story-factory.md
  - _bmad-output/planning-artifacts/product-brief-story-factory-distillate.md
  - docs/story-factory-implementation-spec.md
documentCounts:
  briefs: 2
  research: 0
  brainstorming: 0
  projectDocs: 1
classification:
  projectType: cli_tool
  domain: developer_tooling
  complexity: low
  projectContext: brownfield
workflowType: 'prd'
---

# Product Requirements Document - Story Factory

**Author:** Tom
**Date:** 2026-04-06

## Executive Summary

Story Factory is a Go CLI that automates the mechanical pipeline between BMAD sprint planning and Gastown Beads execution. After a developer completes the creative planning work — PRD, architecture, epic definitions — Story Factory takes the resulting backlog and processes every story through creation, validation, and Beads sync without human intervention. It replaces 12-18 manual Claude CLI invocations per epic with a single command.

The tool orchestrates `claude -p` subprocesses to invoke BMAD's Scrum Master agent for story file generation, re-invokes for validation (auto-accepting all suggestions), and converts each validated story into a Gastown Bead via `bd create`. Stories and epics process sequentially in MVP. Batch runs are resumable — the tool re-reads sprint status between operations and skips stories no longer in `backlog` status.

The desired end state of a Story Factory run: every backlog story has a complete story file, has passed BMAD validation, has its sprint status updated to `ready-for-dev`, and exists as a Bead ready for developer pickup.

### What Makes This Special

No existing tool occupies the BMAD-to-Beads automation niche. AI coding agents help write code; sprint planning SaaS tools help prioritize backlogs. Story Factory does neither — it is the unattended compiler between structured planning output and implementation-ready artifacts. It is BMAD-native (built for the v6 flat `development_status` format), Beads-native (first-class `bd create` integration), and designed for fully unattended operation via `--enable-auto-mode`. The core insight: story validation is almost entirely rubber-stamp, meaning the entire create-validate-sync pipeline can run without human attention.

## Project Classification

- **Type:** CLI Tool — single Go binary, six commands (`create-story`, `validate-story`, `sync-to-beads`, `run`, `epic`, `queue`)
- **Domain:** Developer Tooling / Workflow Automation
- **Complexity:** Low — no regulatory requirements, well-defined integration contracts (BMAD v6 YAML format, Claude CLI streaming JSON, Beads CLI)
- **Context:** Brownfield — fork of `robertguss/bmad_automated`, retaining the Claude subprocess engine (`internal/claude/`), config loading (`internal/config/`), and terminal output (`internal/output/`); rewriting the status parser, removing legacy commands, adding Beads integration

## Success Criteria

### User Success

- Running `story-factory epic <N>` produces fully created, validated, and bead-synced stories for every backlog item in the epic — no manual intervention required
- A structured summary report at completion confirms what landed: stories processed, validation iterations per story, bead IDs created, and any failures with reasons
- The operator's only required action is launching the command and reviewing the summary

### Business Success

- Reduce story processing time from ~30 min/story (manual) to ~5 min/story (automated, amortized across batch runs)
- Eliminate the mechanical overhead of sprint planning — free developer time for creative work (PRD, architecture, epic definition)
- Single binary, zero configuration for standard BMAD project layouts — no adoption barrier

### Technical Success

- >95% of stories complete the full create → validate → sync pipeline without manual intervention
- 100% of stories pass BMAD validation with all suggestions auto-accepted
- Failed stories retry once with the same invocation, then leave artifacts in a recoverable state (partial progress preserved, sprint-status.yaml accurate) before moving on to the next story
- Batch runs are resumable — restarting after a failure skips already-processed stories by re-reading sprint-status.yaml

### Measurable Outcomes

| Metric | Target |
|--------|--------|
| Pipeline completion rate | >95% of stories complete CS→VS→sync unattended |
| Time per story (automated) | ~5 min amortized across batch |
| Validation pass rate | 100% auto-accepted (non-architecture suggestions) |
| Retry success rate | Single retry resolves >50% of transient failures |
| Adoption friction | Zero config for standard BMAD project layout |

## User Journeys

### Journey 1: Epic Batch Processing (Primary — Happy Path)

**Tom, sprint coordinator, Friday afternoon after completing BMAD planning.**

Tom has just finished a week of planning work — the PRD is solid, architecture is reviewed, and epics are defined with all stories listed in `sprint-status.yaml`. Epic 1 has five stories in `backlog` status. Normally, he'd spend the next 2-3 hours invoking Claude CLI once per story to create, once to validate, then manually converting each to a Bead. Instead:

```
$ story-factory epic 1 --project-dir ~/projects/ai-biz-brokerage
```

The tool reads `sprint-status.yaml`, finds five backlog stories for Epic 1, and begins processing sequentially. For each story: invoke `claude -p` to create the story file, re-invoke to validate (auto-accepting suggestions), then run `bd create` to sync to Beads. Between each story, the tool re-reads `sprint-status.yaml` to pick up BMAD's status updates.

Tom walks away. When he comes back, the terminal shows a structured summary:

```
Epic 1: Platform Foundation — 5/5 stories processed
  1-1-frontend-scaffold    ✓ created → validated (1 loop) → bead bd-a1b2
  1-2-database-schema      ✓ created → validated (2 loops) → bead bd-c3d4
  1-3-user-registration    ✓ created → validated (1 loop) → bead bd-e5f6
  1-4-role-based-dashboard  ✓ created → validated (1 loop) → bead bd-g7h8
  1-5-settings-page        ✓ created → validated (1 loop) → bead bd-i9j0

5 created, 5 validated, 5 synced, 0 failed
```

All five story files exist in `_bmad-output/implementation-artifacts/`, all are `ready-for-dev` in sprint-status.yaml, and all have corresponding Beads. The sprint is ready for developers.

### Journey 2: Single Story Processing

**Tom needs to re-process a single story that was added late to the sprint.**

A new story `2-3-payment-integration` was added to `sprint-status.yaml` with `backlog` status after the rest of Epic 2 was already processed. Tom runs:

```
$ story-factory run 2-3-payment-integration --project-dir ~/projects/ai-biz-brokerage
```

The tool verifies the story exists and is in `backlog`, then runs the full pipeline for just that one story: create → validate → sync. Tom watches the output in real time with `--verbose` because this story is complex and he wants to see what BMAD generates.

### Journey 3: Full Queue Processing

**Tom has completed planning for a new project with three epics and 14 stories total.**

Rather than processing epic by epic, he fires the full queue:

```
$ story-factory queue --project-dir ~/projects/new-project
```

The tool reads `sprint-status.yaml`, finds all 14 backlog stories across three epics, and processes them sequentially — Epic 1 stories first (in key order), then Epic 2, then Epic 3. Each story goes through the full pipeline. The final summary groups results by epic.

### Journey 4: Failure Recovery

**Mid-way through processing Epic 2, Claude CLI times out on story `2-2-elevenlabs-voice-integration`.**

The tool retries the failed step once. The retry also fails (API is having a bad day). The tool logs the failure, leaves `2-2` in `backlog` status (story file was never created, so sprint-status.yaml is accurate), and moves on to `2-3`. When the batch completes, the summary shows:

```
Epic 2: Seller Onboarding — 3/4 stories processed
  2-1-ai-advisor-conversation  ✓ created → validated → bead bd-k1l2
  2-2-elevenlabs-voice          ✗ FAILED: create-story timed out (retried 1x)
  2-3-payment-integration       ✓ created → validated → bead bd-m3n4
  2-4-seller-profile            ✓ created → validated → bead bd-o5p6

3 created, 3 validated, 3 synced, 1 failed
```

Later, when Claude is healthy again, Tom runs `story-factory run 2-2-elevenlabs-voice-integration`. The tool picks up the story in `backlog` status and processes it normally. No manual cleanup was needed.

### Journey 5: Dry Run / Planning

**Tom wants to see what a queue run would do before committing.**

```
$ story-factory queue --dry-run --project-dir ~/projects/ai-biz-brokerage
```

The tool reads `sprint-status.yaml` and reports what it would process without executing anything:

```
Dry run — no changes will be made

Epic 1: 3 backlog stories would be processed
  1-3-user-registration (backlog → create → validate → sync)
  1-4-role-based-dashboard (backlog → create → validate → sync)
  1-5-settings-page (backlog → create → validate → sync)

Epic 2: 4 backlog stories would be processed
  2-1-ai-advisor-conversation (backlog → create → validate → sync)
  ...

Total: 7 stories across 2 epics
```

Tom sees that stories `1-1` and `1-2` are already `ready-for-dev` (processed in a prior run) and confirms the queue will only touch the remaining backlog items.

### Journey Requirements Traceability

These five journeys map directly to the Functional Requirements (FR1–FR36). Key capability areas: sprint status parsing (FR1–FR6), story creation (FR7–FR9), validation (FR10–FR13), Beads sync (FR14–FR18), pipeline orchestration (FR19–FR27), preconditions (FR28–FR31), and reporting (FR32–FR36).

## CLI-Specific Requirements

### Command Structure

Six commands composing a three-step pipeline:

| Command | Purpose | Input | Precondition |
|---------|---------|-------|--------------|
| `create-story <key>` | Invoke BMAD to generate story file | Story key | Status is `backlog` |
| `validate-story <key>` | Re-invoke BMAD in validation mode | Story key | Status is `ready-for-dev`, story file exists |
| `sync-to-beads <key>` | Convert story to Gastown Bead | Story key | Status is `ready-for-dev`, story file exists |
| `run <key>` | Full pipeline for one story | Story key | Status is `backlog` |
| `epic <N>` | Full pipeline for all backlog stories in epic | Epic number | At least one `backlog` story in epic |
| `queue` | Full pipeline for entire backlog | None | At least one `backlog` story exists |

**Global flags:**
- `--project-dir <path>` — Project root (default: CWD)
- `--dry-run` — Show planned operations without executing
- `--verbose` — Stream Claude CLI output in real time

### Output Formats

- **Terminal output (default):** Formatted progress indicators during processing, structured summary table on completion showing per-story status, validation loop count, bead IDs, and failure reasons
- **Claude consumption:** Tool reads `--output-format stream-json` from Claude subprocess internally; not exposed to user
- **Dry run output:** Plain text listing of planned operations grouped by epic

### Configuration

- Viper-based config with `BMAD_` env var prefix (inherited from fork)
- Minimal config surface for MVP: prompt template and default project paths
- `--project-dir` flag overrides config for non-CWD projects
- No config file required for standard BMAD project layouts (zero-config default)

### Scripting Support

- All commands are non-interactive — no prompts, no stdin required
- Exit codes: `0` success, `1` partial failure (some stories failed), `2` fatal error (precondition not met)
- Designed for unattended execution — `--enable-auto-mode` passed to all Claude invocations
- Resumable by design — re-running the same command skips already-processed stories

### Implementation Considerations

- Each Claude invocation is a fresh `claude -p` subprocess — no context carryover between steps or stories
- Sprint-status.yaml must be re-read between story operations (BMAD modifies it after each create-story)
- Story file path derived from `story_location` field in sprint-status.yaml with `{project-root}` placeholder resolved
- Precondition checks run before any processing: `bd` on PATH, sprint-status.yaml exists, BMAD agent files present
- Single retry on any step failure, then mark failed and continue batch

## Product Scope

### MVP Strategy & Philosophy

**MVP Approach:** Problem-solving MVP — eliminate the manual overhead of BMAD story processing with the thinnest viable automation layer. No feature is included unless it directly serves the unattended create → validate → sync pipeline.

**Resource Requirements:** Solo developer, single Go binary, no infrastructure dependencies beyond Claude CLI and Beads CLI already on PATH.

### MVP Feature Set (Phase 1)

**Core User Journeys Supported:**
- All five mapped journeys: epic batch, single story, full queue, failure recovery, dry run

**Must-Have Capabilities:**
- Six commands: `create-story`, `validate-story`, `sync-to-beads`, `run`, `epic`, `queue`
- Sequential processing (stories and epics one at a time)
- Pipeline-as-unit semantics: all three steps (create, validate, sync) must pass for a story to be considered complete; failure at any step marks the entire story as failed
- Single retry on any step failure, then skip and continue batch
- Validation loop: auto-accept all suggestions, max 3 iterations, mark `needs-review` on exhaustion (no architecture/config keyword detection — keep it simple)
- `--dry-run`, `--verbose`, `--project-dir` flags
- `--enable-auto-mode` on all Claude invocations
- Precondition checks before processing
- Structured terminal summary on completion
- Resumable batch runs via sprint-status.yaml re-read

### Post-MVP Features

**Phase 2 (Growth):**
- Parallel epic processing via tmux (stories sequential within each epic, epics concurrent)
- Configurable concurrency limit for parallel processing
- `--blocks` dependency chaining in Beads sync
- Run summary exported as JSON file
- Smarter validation escalation (keyword-based detection of architecture/config suggestions)

**Phase 3 (Expansion):**
- CI/CD-triggered backlog processing on PRD/architecture merge
- Validation feedback loop surfacing recurring issues to upstream planning
- Integration with BMAD retrospective workflow
- Multi-project orchestration
- Cross-epic dependency-aware story ordering

### Risk Mitigation Strategy

**Technical Risks:** Claude CLI timeouts and API instability are the primary concern. Mitigated by per-story process isolation, single retry, and resumable batch runs that skip already-processed stories. Each story is a fresh subprocess — one failure cannot corrupt another.

**Integration Risks:** BMAD v6 format changes could break the status parser. Mitigated by parser tests against real sprint-status.yaml samples and loud failure on unexpected format. `bd create` output format changes mitigated by parsing only the bead ID from stdout.

**Resource Risks:** Solo developer project — scope is deliberately minimal. No parallel processing, no web UI, no multi-project support in v1. If further cuts were needed, they won't be — the MVP is already lean.

## Functional Requirements

### Sprint Status Management

- FR1: Operator can read and parse a BMAD v6 flat `development_status` YAML map from `sprint-status.yaml`
- FR2: Operator can look up a specific story entry by its key and retrieve its current status
- FR3: Operator can retrieve all stories with a given status (e.g., all `backlog` stories)
- FR4: Operator can retrieve all stories belonging to a specific epic by epic number
- FR5: System can classify each entry as epic (`epic-N`), story (`N-M-slug`), or retrospective (`epic-N-retrospective`)
- FR6: System can resolve the `story_location` path template by replacing `{project-root}` with the actual project directory

### Story Creation

- FR7: Operator can invoke story creation for a single story by key, which spawns a `claude -p` subprocess with `--enable-auto-mode --output-format stream-json`
- FR8: System can verify that a story file was created at the expected path after creation completes
- FR9: System can re-read `sprint-status.yaml` after creation to confirm status changed from `backlog` to `ready-for-dev`

### Story Validation

- FR10: Operator can invoke story validation for a single story by key, using the same `claude -p` command (BMAD auto-detects validation mode when story file exists)
- FR11: System can auto-accept all improvement suggestions during validation
- FR12: System can re-validate after accepting suggestions, up to a maximum of 3 iterations
- FR13: System can mark a story as `needs-review` if validation does not converge after 3 iterations

### Beads Synchronization

- FR14: Operator can sync a validated story to Gastown Beads by invoking `bd create` with the story title and acceptance criteria
- FR15: System can extract the story title from the first `# Story X.Y: Title` heading in the story markdown
- FR16: System can extract acceptance criteria from the content between `## Acceptance Criteria` and the next `##` heading
- FR17: System can parse the bead ID from `bd create` stdout
- FR18: System can append a `<!-- bead:{bead-id} -->` tracking comment to the story file after successful sync

### Pipeline Orchestration

- FR19: Operator can run the full pipeline (create → validate → sync) for a single story with the `run` command
- FR20: Operator can run the full pipeline for all backlog stories in an epic with the `epic` command
- FR21: Operator can run the full pipeline for all backlog stories across all epics with the `queue` command
- FR22: System processes stories sequentially within an epic, in key order
- FR23: System processes epics sequentially in numeric order (MVP)
- FR24: System re-reads `sprint-status.yaml` between story operations to reflect BMAD's updates
- FR25: System skips stories that are no longer in `backlog` status, enabling resumable batch runs
- FR26: System retries a failed step once before marking the story as failed and continuing the batch
- FR27: System treats the pipeline as a unit — failure at any step (create, validate, or sync) marks the entire story as failed

### Precondition Verification

- FR28: System can verify that `bd` CLI is available on PATH before processing
- FR29: System can verify that `sprint-status.yaml` exists at the expected location before processing
- FR30: System can verify that BMAD agent files are present before processing
- FR31: System exits with a fatal error (exit code 2) if any precondition fails

### Reporting & Output

- FR32: Operator can run any batch command with `--dry-run` to see planned operations without executing
- FR33: Operator can run any command with `--verbose` to stream Claude CLI output in real time
- FR34: Operator can specify a non-CWD project directory with `--project-dir`
- FR35: System displays a structured summary on batch completion showing: stories processed, validation iterations per story, bead IDs created, and failures with reasons
- FR36: System exits with code 0 on full success, code 1 on partial failure, and code 2 on fatal precondition error

## Non-Functional Requirements

### Integration

- NFR1: System must be compatible with Claude CLI's `--output-format stream-json` streaming protocol and `--enable-auto-mode` flag
- NFR2: System must parse BMAD v6 flat `development_status` YAML format; reject unrecognized formats with a clear error message rather than silently producing wrong results
- NFR3: System must parse `bd create` stdout to extract bead IDs; fail gracefully with a clear error if `bd` output format is unrecognized
- NFR4: System must treat `sprint-status.yaml` as an external mutable resource — always re-read from disk before each story operation, never cache stale state

### Reliability

- NFR5: A failure in one story's pipeline must not affect processing of subsequent stories in a batch run
- NFR6: System must never leave `sprint-status.yaml` in a corrupted state — since Story Factory only reads this file (BMAD writes it), this is enforced by design
- NFR7: System must not leave orphaned Claude subprocesses on crash or timeout — subprocess cleanup must be handled on all exit paths
- NFR8: After any failure, the system state must allow the same command to be re-run without manual cleanup (idempotent resumability)

### Performance

- NFR9: Claude subprocess timeout must be configurable, with a default sufficient for complex story generation (recommended: 5 minutes per invocation)
- NFR10: Tool startup and precondition checks must complete in under 2 seconds — operator should see feedback immediately
