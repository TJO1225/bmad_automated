# Story Factory — Project Status (2026-04-17)

**Dispatch / PR / parallel resume:** [docs/DISPATCH_AND_PR_HANDOFF.md](docs/DISPATCH_AND_PR_HANDOFF.md)

## Story Factory Tool Status

Binary at `8666708` on `main`. Installed globally via `go install`. All tests pass (`just check`).

### Key commits shipped:
- Phase 1: Mode-driven pipeline (bmad/beads), validate removed, dev-story + code-review steps
- Phase 2: commit-branch + open-pr steps, git/storyfile packages
- Phase 3: tmux dispatcher, git worktrees, parallel execution
- Phase 4: Resume semantics (skip already-completed steps), `IsProcessable` status filter
- Fixes: non-retryable dev-story, flexible title parser, lenient code-review post-condition

### Known limitations:
- BMAD's code-review skill doesn't always flip sprint-status to `done` in `claude -p` mode — story-factory treats clean exit as success regardless
- BMAD skills requiring interactive MCP auth (e.g. Supabase OAuth) fail in `claude -p` — must be done interactively first
- Shell line breaks in long `dispatch` commands cause partial execution — use file-based key lists or argless dispatch

---

## Vora Website (`~/dev_wsl/Vora/vora-website`)

**Repo:** `github.com/TJO1225/vora-website` | **Branch:** `main`

**Authoritative status:** `_bmad-output/implementation-artifacts/sprint-status.yaml` (updated 2026-04-17 in repo copy). **Dispatch / PR / resume playbook:** [docs/DISPATCH_AND_PR_HANDOFF.md](docs/DISPATCH_AND_PR_HANDOFF.md) (includes a **copy-paste handoff prompt** for another agent).

### Sprint snapshot (YAML-driven; reconcile after merges)

| Area | Notes |
|------|--------|
| Epics 1–6, most of 7–8 | `done` through admin shell/dashboard; **7-8** `done`, **7-9** `review` (epic-7 still `in-progress` until 7-9 closes). |
| Epic 8 | **8-1** `review`; **8-9** `ready-for-dev`; rest in that epic largely `done`. |
| Epics 9–13 | Many **`ready-for-dev`** rows; **backlog** tails (e.g. 9-6, 11-6–11-7, 12-5–12-7, 13-4–13-9); **13-2**, **13-3** `done`. |

### Dispatch on this machine

- Check **tmux** windows `sf-batch-*` on the host where you started dispatch; scrollback shows per-story exit and `__SF_DONE__:…`.  
- **Argless** `story-factory dispatch --parallel N` schedules **all** unfinished stories—often a long queue. Prefer **explicit keys** for the next wave (see handoff doc).  
- **Precondition:** primary clone **`git status`** clean on `main` before starting a new dispatch.  
- **Worktrees:** `.story-factory/worktrees/<key>/` (may be empty locally if dispatch runs on another host). Run **`story-factory cleanup`** after PRs merge.

### PRs

Run `cd ~/dev_wsl/Vora/vora-website && gh pr list --state open --limit 100` on the machine with `gh` auth; merge or fix conflicts per [DISPATCH_AND_PR_HANDOFF.md](docs/DISPATCH_AND_PR_HANDOFF.md).

---

## BizTender MVP (`~/dev_wsl/Biz-Brokerage/ai-biz-brokerage`)

**Repo:** `github.com/TJO1225/ai-biz-brokerage` | **Branch:** `main`

### Sprint summary:
| Status | Count | Notes |
|--------|-------|-------|
| done | 2 | 6-4, 12-5a |
| review | 41 | Need code-review → commit → PR |
| ready-for-dev | 14 | Need dev-story → review → commit → PR |
| deferred-post-mvp | 3 | 7-3, 7-4, 7-5 (skipped by story-factory) |
| backlog | 11 | Epics 14/15 (post-MVP, don't touch) |

### What ran:
- **12-5a** completed end-to-end: code-review → commit → PR #2 → merged
- **12-5b** succeeded: PR #3 → merged
- **12-5d** succeeded: PR #4 → merged
- **13-2** succeeded: PR opened

### Currently dispatching (3 parallel):
- 13-3-financial-decimal-boundary-tests
- 13-4-cross-tenant-authorization-tests
- 13-5-frontend-component-test-backfill
- 13-6-e2e-golden-path-tests

### Failed stories to investigate:
- `12-5c-quarantine-hotel-orchestration` — exit 2 (precondition failure, dirty worktree from prior run). Needs `story-factory cleanup --force` on that worktree, then retry.
- `13-1-auth-and-rls-integration-test-suite` — exit 1 (story-level failure, dev or review didn't complete)

### Remaining work after current dispatch:
- **41 review-state stories** across Epics 1-12 still need code-review passes. These are the MVP bottleneck (Story 12.7 gate requires review artifacts). Can dispatch them in batches once Epic 13 stories land.
- **8 ready-for-dev stories** in Wave 2 tail + Wave 3 patches (1-5, 2-7, 2-8, 5-5, 11-3, plus any remaining)

### Worktrees:
Active at `.story-factory/worktrees/`. Run `story-factory cleanup --force` to clear failed ones before retrying.

---

## Next actions when resuming

### Vora:
1. Follow [docs/DISPATCH_AND_PR_HANDOFF.md](docs/DISPATCH_AND_PR_HANDOFF.md) (tmux → PR list → merge → sprint YAML → cleanup → next dispatch).
2. Prefer **explicit story keys** for the next `dispatch` wave so 40+ unfinished rows do not all enqueue at once unless intended.

### BizTender:
1. Check dispatch results: `cd ~/dev_wsl/Biz-Brokerage/ai-biz-brokerage && gh pr list`
2. Merge landed PRs
3. Investigate 12-5c and 13-1: `story-factory cleanup --force` then `story-factory run <key> --verbose`
4. Start code-review sweep on the 41 review-state stories: `story-factory dispatch --parallel 3`
5. After review stories complete, tackle the 8 remaining ready-for-dev stories

### Story Factory tool improvements to consider:
- Fix 3: Add MCP-avoidance prompt (tell BMAD to skip interactive auth MCPs in `claude -p`)
- Batch merge helper: `story-factory merge-all` to auto-merge open PRs sequentially
- Sprint-status auto-flip: story-factory updates status to `done` after successful code-review instead of relying on BMAD
