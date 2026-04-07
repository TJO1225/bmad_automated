---
title: "Product Brief: Story Factory"
status: "complete"
created: "2026-04-06"
updated: "2026-04-06"
inputs:
  - docs/story-factory-implementation-spec.md
  - https://github.com/gastownhall/beads
  - _bmad/bmm/config.yaml
---

# Product Brief: Story Factory

## Executive Summary

Story Factory is a Go CLI that automates the repetitive bulk of BMAD sprint planning. After a developer completes the creative work — PRD, architecture, epic definitions — Story Factory takes the backlog and factory-lines every story through creation, validation, and implementation handoff, all without human intervention.

The tool orchestrates Claude CLI invocations to have BMAD's Scrum Master agent generate full story files, then re-invokes to validate each story (auto-accepting all improvement suggestions), and finally converts validated stories into Gastown Beads for developer pickup. It runs stories in parallel via tmux, retries on failure, and uses Claude's `--enable-auto-mode` for fully unattended operation. One command can process an entire epic or the full backlog.

Story Factory is the compiler between BMAD planning and Beads execution — two systems that don't talk to each other today. It turns a manual, per-story, multi-step process into a single command that runs while you do something else.

## The Problem

BMAD planning produces a sprint backlog — a list of story keys in `sprint-status.yaml`. Turning that backlog into implementation-ready story files requires invoking Claude CLI once per story to create, once to validate, then manually converting each to a Gastown bead. For an epic with 4-6 stories, that's 12-18 manual invocations with wait time between each.

Today this is done by hand: launch Claude, paste the prompt, wait for output, check the result, repeat. Each invocation needs a clean context (separate process), and the sprint status file gets updated by BMAD after each creation — so you have to re-read it between stories. The overhead scales linearly with backlog size and is entirely mechanical. It's the kind of work that makes a developer question their life choices.

The validation step adds friction: BMAD's validator surfaces suggestions that should be accepted, but doing so requires human attention on each one. Without auto-acceptance, validation becomes a bottleneck rather than a quality gate.

## The Solution

Story Factory is a CLI binary (`story-factory`) with six commands that compose a three-step pipeline:

**Core pipeline (per story):**
1. **Create** — Invokes `claude -p "bmad-bmm-create-story {key}" --enable-auto-mode --output-format stream-json`. BMAD's Scrum Master reads the PRD, architecture, and epics, then generates a complete story file. BMAD updates sprint-status.yaml automatically.
2. **Validate** — Re-invokes the identical command. BMAD detects the existing story file and switches to validation mode, running its checklist. The orchestration layer auto-accepts improvement suggestions and re-validates (max 3 iterations). Major issues — those indicating PRD or architecture-level problems — are escalated to the operator rather than auto-accepted.
3. **Sync to Beads** — Extracts the title and acceptance criteria from the story markdown, runs `bd create`, and appends a `<!-- bead:{id} -->` tracking comment back to the story file.

**Batch commands:**
- `story-factory epic <N>` — Processes all backlog stories in an epic sequentially (to avoid sprint-status.yaml write contention). Parallel execution applies across epics when running `queue`.
- `story-factory queue` — Processes the entire backlog, running epics in parallel via tmux (stories sequential within each epic).

**Resilience:** Built-in retry logic with exponential backoff handles Claude failures and transient errors. The tool re-reads `sprint-status.yaml` between operations since BMAD modifies it after each story creation. Batch runs are resumable — the tool skips stories that are no longer in `backlog` status, so restarting after a partial failure picks up where it left off.

**Observability:** Each run produces a structured summary: stories processed, validation loops per story, failures with reasons, and bead IDs created. `--verbose` streams Claude output in real time per tmux pane.

## What Makes This Different

No existing tool occupies this niche. AI coding agents (Claude Code, Aider, Kiro) help you write code. Sprint planning SaaS tools (SprintiQ, Jira AI) help estimate and prioritize. Story Factory does neither — it's a **planning-to-execution compiler** that takes structured BMAD output and produces implementation-ready artifacts with zero human intervention.

Key differentiators:
- **BMAD-native** — Designed specifically for BMAD v6's flat `development_status` format and agent contracts
- **Beads-native** — First-class integration with Gastown's dependency-aware task tracker
- **Parallel execution** — tmux-based concurrency across epics; sequential within each epic to avoid YAML contention
- **Unattended operation** — `--enable-auto-mode` means zero human intervention from backlog to beads
- **Validation with teeth** — Auto-accepts minor suggestions, escalates major issues to the operator, and re-validates until clean — ensuring story quality not just story existence

## Who This Serves

**Primary:** Developers and sprint coordinators using the BMAD methodology who have completed planning and need to efficiently convert a backlog of epic/story definitions into implementation-ready artifacts and Beads tasks.

**The "aha moment":** Running `story-factory epic 1` and coming back to find 4-6 fully created, validated, and bead-synced stories — work that previously took an afternoon of manual invocations.

## Success Criteria

| Metric | Target |
|--------|--------|
| Time saved per sprint | Reduce story processing from ~30 min/story (manual) to ~5 min/story (automated, amortized across parallel runs) |
| Story quality | 100% of stories pass BMAD validation with all suggestions accepted — no manual review needed |
| Pipeline reliability | >95% of stories complete the full CS→VS→sync pipeline without manual intervention |
| Adoption friction | Single binary, zero config for standard BMAD project layout |

## Scope

**v1 (MVP):**
- Six commands: `create-story`, `validate-story`, `sync-to-beads`, `run`, `epic`, `queue`
- Parallel epic processing via tmux (stories sequential within each epic)
- Retry logic for Claude and bd failures
- `--enable-auto-mode` for unattended execution
- `--dry-run` and `--verbose` flags
- `--project-dir` for non-CWD projects
- Flat sprint-status.yaml parser (BMAD v6 format)
- Validation loop: auto-accept minor suggestions, escalate major issues, re-validate until clean (max 3 iterations)
- Precondition checks: verify `bd` is on PATH, sprint-status.yaml exists, BMAD agent files are present before processing
- Structured run summary (stories processed, failures, bead IDs)
- Resumable batch runs (skips non-backlog stories on restart)

**Explicitly out of scope for v1:**
- Cross-epic dependency awareness (story ordering beyond key order)
- CI/CD integration or scheduled runs
- Web UI or dashboard
- Multi-project orchestration
- Story content customization (BMAD agents own the content)

## Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| BMAD v6 format changes break the status parser | High | Pin to known BMAD version; parser tests against real sprint-status.yaml samples; fail loudly on unexpected format |
| `--enable-auto-mode` causes unintended side effects | High | Validate post-conditions after each step (file exists, status changed, bead created); abort pipeline on unexpected state |
| Claude CLI flakes on long batch runs | Medium | Retry with exponential backoff; per-story isolation (each invocation is a fresh process); report partial progress |
| Parallel tmux sessions create race conditions on sprint-status.yaml | Medium | Stories within an epic run sequentially; parallelism only across epics (separate YAML key spaces) |
| Validation loop fails to converge (suggestions keep cycling) | Medium | Cap at 3 iterations; on exhaustion, mark story as needs-review and continue batch |
| Partial pipeline completion (created but not synced to Beads) | Medium | Resumable runs detect partial state; `bd` availability checked as precondition |

## Vision

If Story Factory works well, it becomes the execution engine for the BMAD methodology — not just story creation, but the full lifecycle from planning completion to developer handoff. Future directions include CI-triggered backlog processing on PRD/architecture merge, a feedback loop where recurring validation failures surface upstream planning issues, integration with BMAD's retrospective workflow, and expansion to other BMAD outputs (design reviews, post-mortems). The long-term play: planning becomes programming, and Story Factory is the compiler.
