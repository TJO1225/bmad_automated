---
title: "Product Brief Distillate: Story Factory"
type: llm-distillate
source: "product-brief-story-factory.md"
created: "2026-04-06"
purpose: "Token-efficient context for downstream PRD creation"
---

# Product Brief Distillate: Story Factory

## Requirements Hints

- CLI binary name: `story-factory` (rename from `bmad-automate`)
- Six commands: `create-story`, `validate-story`, `sync-to-beads`, `run`, `epic`, `queue`
- `--project-dir <path>` flag (default: CWD) to point at any BMAD project
- `--dry-run` flag shows planned steps without executing (which stories, what status, what transitions)
- `--verbose` flag streams Claude output in real-time per tmux pane
- Claude invocation uses `--enable-auto-mode` (replaces `--dangerously-skip-permissions` from original spec) + `--output-format stream-json`
- Single prompt template: `bmad-bmm-create-story {story-key}` — BMAD agents handle all context loading internally
- Each Claude invocation is a fresh subprocess (`claude -p`) — no context carryover between steps
- Validation loop: auto-accept minor suggestions, escalate major/architectural issues to operator, re-validate until clean, max 3 iterations, mark as `needs-review` on exhaustion
- Stories within an epic process sequentially (avoids sprint-status.yaml write contention); epics process in parallel via tmux
- Batch runs are resumable: tool reads sprint-status.yaml and skips stories no longer in `backlog` status
- Precondition checks before processing: verify `bd` on PATH, sprint-status.yaml exists, BMAD agent files present
- Structured run summary on completion: stories processed, validation loops per story, failures with reasons, bead IDs created
- tmux sessions need naming conventions and cleanup-on-exit to prevent orphaned sessions on crash

## Sprint-Status.yaml Format (BMAD v6)

- Flat `development_status` map under top-level YAML — NOT nested `sprints[].stories[]`
- Epic entries: key `epic-N`, value is status string
- Story entries: key `N-M-slug` (N=epic number, M=story number, slug=description), value is status string
- Retrospective entries: key `epic-N-retrospective`, value `optional` or `done`
- Comments with `#` provide epic titles (display only, not parsed)
- Status values: `backlog`, `ready-for-dev`, `in-progress`, `review`, `done`, `optional`
- Story Factory only reads this file — BMAD writes it (story `backlog→ready-for-dev`, first-in-epic also sets epic `backlog→in-progress`)
- Tool must re-read YAML between story operations because BMAD modifies it after each create-story
- `story_location` field contains path template with `{project-root}` placeholder

## Story File Contract

- Output path: `_bmad-output/implementation-artifacts/{story-key}.md` (flat, not nested)
- Filename matches sprint-status.yaml key exactly
- Title extractable from first `# Story X.Y: Title` heading
- Acceptance criteria extractable from content between `## Acceptance Criteria` and next `##` heading
- After bead sync, file gets `<!-- bead:{bead-id} -->` comment appended for bidirectional tracking

## Beads Integration Contract

- Invocation: `bd create "{story-key}: {extracted-title}" --notes "{acceptance-criteria-text}"`
- Bead ID parsed from `bd create` stdout (hash-based, e.g., `bd-a1b2`)
- Beads uses Dolt backend (version-controlled SQL); supports hierarchical IDs (`bd-a3f8.1`)
- `bd` must be on PATH and initialized in repo (`bd init` already run)
- No `--blocks` dependency chaining in v1 — future enhancement
- Failure modes to handle: bd not installed, bd not initialized, bd rejecting acceptance criteria format

## Technical Context

- Language: Go — existing fork requirement, single binary distribution
- Fork of `robertguss/bmad_automated` by robertguss — being stripped down and retooled
- Dev machine: SP9, Windows 11 WSL2, hostname Genie, user tjo12
- Repo location on SP9: `~/dev_wsl/clawsqwad/bmad_automated`
- Build system: `just` task runner (justfile recipes for build, test, lint, run)
- Dependencies: spf13/cobra (CLI), go-yaml (YAML), lipgloss (terminal styling), Viper (config)
- Keep packages: `internal/claude/` (subprocess + JSON streaming), `internal/config/` (Viper), `internal/output/` (terminal formatting)
- Rewrite packages: `internal/status/` (new flat YAML parser)
- Remove packages: `internal/lifecycle/`, `internal/state/`, `internal/router/`, removed CLI commands (dev-story, code-review, git-commit, full-cycle)
- Add packages: `internal/beads/` (bd create integration)
- Key interface: `claude.Executor` (mockable for tests), `output.Printer` (capturable for tests)

## Rejected Ideas & Decisions

- **Nested sprints[] YAML format** — rejected; BMAD v6 uses flat `development_status` map; original fork's parser is incompatible
- **Separate validation prompt** — rejected; same `bmad-bmm-create-story` prompt is reused; BMAD auto-detects validation mode when story file exists
- **Story Factory writes to sprint-status.yaml** — rejected; BMAD handles all writes; Story Factory is read-only
- **Cross-epic dependency ordering in v1** — rejected; stories process in key order within epic; dependency awareness is a future enhancement
- **CI/CD integration in v1** — rejected; out of scope; future direction for automated backlog processing on PRD merge
- **Token cost tracking** — rejected for v1; over-engineering for an internal tool; accept the cost
- **Parallel story creation within an epic** — rejected; YAML write contention risk too high; serialize within epic, parallelize across epics
- **Web UI or dashboard** — rejected for v1; CLI-only
- **`--dangerously-skip-permissions`** — replaced by `--enable-auto-mode` for cleaner unattended operation

## Competitive Landscape

- **AI coding agents** (Claude Code, Aider, Kiro, Crush): help write code, not plan sprints — different layer entirely
- **Sprint planning SaaS** (SprintiQ, Taskade, Jira AI, Miro AI, Spinach): estimate, prioritize, and manage backlogs in web UIs — not CLI tools, not BMAD-native, don't generate story files
- **Claude CLI orchestration patterns** (claude-code-workflows, agent-workflow plugins): generic agent orchestration — not BMAD-specific, no Beads integration, no sprint-status awareness
- **No direct competitor** occupies the BMAD planning → story file → Beads handoff niche
- Market trend: long-running autonomous agent workflows are accelerating in 2026; `claude -p` pipeline automation is becoming standard practice

## Open Questions

- How should "major issue" be defined for validation escalation? Keyword-based detection? BMAD severity tags? Needs specification during PRD.
- What's the concurrency limit for parallel epic processing via tmux? Configurable flag or hardcoded cap?
- Should `queue` command process epics in numeric order, or allow priority ordering?
- What happens if `bd create` output format changes? Need a parsing contract or version pin.
- Should the run summary be written to a file (JSON report) in addition to terminal output?

## Scope Signals

- **In for v1:** All six commands, sequential-within-epic processing, parallel-across-epic via tmux, retry logic, `--enable-auto-mode`, validation with escalation, resumable batch runs, precondition checks, structured summary
- **Out for v1:** Cross-epic dependencies, CI/CD triggers, web UI, multi-project, story content customization, token tracking
- **Maybe for v2:** CI/CD integration on PRD merge, validation feedback loop to upstream planning, retrospective workflow integration, `--blocks` dependency chaining in Beads sync, run summary as JSON file
