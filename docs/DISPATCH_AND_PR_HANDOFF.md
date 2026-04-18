# Dispatch, parallel stories, and PR handoff

This document describes how **`story-factory dispatch`** fits together with **git worktrees**, **tmux**, **sprint-status**, and **GitHub PRs**, and gives a **copy-paste handoff prompt** for another session (or teammate) to triage PRs without touching a running dispatch.

---

## Copy-paste: handoff prompt for PR / dispatch triage

Use this in Cursor (or any agent) on the machine where the repos and `gh` auth live. Adjust `PROJECT_ROOT` and org/repo if needed.

```text
You are helping resume BMAD / story-factory work.

Context:
- story-factory: https://github.com/…/bmad_automated (or local path). Docs: bmad_automated/docs/DISPATCH_AND_PR_HANDOFF.md
- Do NOT rebuild or reinstall story-factory if a tmux dispatch is still running (panes were started with the existing binary path).

Tasks (in order):

1) **Dispatch health (tmux)**  
   - Run `tmux ls` and attach to the session that was used for dispatch (`tmux attach -t <name>`).  
   - Look for windows named `sf-batch-1`, `sf-batch-2`, …  
   - In each pane, scrollback should show `story-factory run …` and a line like `__SF_DONE__:<nonce>:<exit>`. Note any exit code `1`.  
   - When the supervisor process exits, the terminal that launched dispatch prints a **Dispatch summary: N succeeded, M failed**.

2) **Open PRs (GitHub)**  
   For each active product repo, from repo root:
   ```bash
   cd <PROJECT_ROOT>
   gh pr list --state open --limit 100
   gh pr status
   ```
   For each open PR: note **mergeable** vs **conflicts**, CI state, and whether it duplicates another branch.

3) **Merge policy (human or CLI)**  
   - Prefer **Squash merge** on GitHub when green.  
   - If **not mergeable**: merge or rebase `origin/main` into the PR branch, push, then merge.  
   - To avoid local worktree errors when many `.story-factory/worktrees/*` exist, merge from GitHub UI, or use:
     `gh pr merge <N> --squash --repo OWNER/REPO`  
     with cwd **outside** any story worktree (e.g. `cd /tmp` first), or merge in the **primary** clone only.

4) **Sprint truth**  
   Open `_bmad-output/implementation-artifacts/sprint-status.yaml` and reconcile story rows with reality (PR merged → often `done`; story stuck in `review` → run `story-factory run <key> --verbose` once from a clean worktree or primary clone).

5) **Worktree cleanup**  
   After PRs merge:
   ```bash
   cd <PROJECT_ROOT>
   story-factory cleanup
   ```
   Use `story-factory cleanup --force` only to drop **unmerged** worktrees you are sure you do not need (destructive).

6) **Preconditions before the next dispatch**  
   From **project root** (primary clone): `git status` must be **clean** on the branch dispatch uses (usually `main`). Commit or stash local changes.  
   Then inside tmux:  
   `story-factory dispatch --parallel 4 --project-dir <PROJECT_ROOT>`

Deliverables: a short table (Story key | sprint row | PR # | mergeable? | next action) and any **failed** dispatch keys with the last error line from the pane.
```

---

## How `dispatch` works (mental model)

| Piece | Role |
|--------|------|
| **tmux** | Required. Dispatch refuses to run outside a tmux session. |
| **Windows `sf-batch-*`** | Up to 4 panes per window; `--parallel N` caps concurrent stories. |
| **`.story-factory/worktrees/<story-key>/`** | One git worktree per story; **`story-factory run`** uses `--project-dir` pointing at that path. |
| **`os.Executable()`** | Each pane runs the **same** `story-factory` binary path as the process that started `dispatch`. Reinstalling mid-flight does not change already-running panes; new installs affect **the next** dispatch. |
| **Story list (no args)** | All **unfinished** stories in `sprint-status.yaml`: `backlog`, `ready-for-dev`, `in-progress`, `review` (not `done`, not custom statuses like `deferred-post-mvp`). Same set conceptually as `queue` without args. |
| **Explicit args** | `story-factory dispatch key1 key2 …` runs exactly those keys (order preserved). |
| **Resume** | If a worktree already exists, dispatch **reuses** it; each `run` skips completed pipeline steps based on story status and artifacts. |
| **End of batch** | Summary + reminder: run **`story-factory cleanup`** after merges. |

Preconditions (`story-factory dispatch` / `run` / …): include a **clean git working tree** on the resolved project directory, BMAD agent paths, `sprint-status.yaml`, and (in bmad mode) **`gh`** on `PATH`. A **dirty primary clone** yields exit **2** before any story starts.

---

## PR automation outside a running dispatch

- **Triage**: `gh pr list`, open in browser, fix merge conflicts locally on the PR branch, push.  
- **Merge**: GitHub “Squash and merge”, or `gh pr merge` as above.  
- **`review-pr` pipeline step** (optional last step in bmad mode): second LLM reviews the PR URL then runs `gh pr merge`. Failures there do **not** remove commits already pushed; they only block automated merge. You can remove `review-pr` from `config/workflows.yaml` for a project copy if you prefer manual merges only.

---

## vora-website: where things stand (sprint file snapshot)

Paths:

- Repo: `~/dev_wsl/Vora/vora-website` (remote typically `TJO1225/vora-website`).
- Sprint: `_bmad-output/implementation-artifacts/sprint-status.yaml`.

Epic 7 is nearly closed: **`7-9-smoke-test-deploy-auth-boundary`** is in **`review`** (last epic-7 story before epic done). Epic 8–13 have a mix of **done**, **review** (`8-1-resend-client-email-templates`), **ready-for-dev**, and **backlog** tails (e.g. `9-6`, `11-6`, `11-7`, `12-5`–`12-7`, `13-4`–`13-9`).

**Argless** `dispatch` will enqueue **every** unfinished row in sort order—often dozens of stories. To **accelerate in waves**, pass explicit keys, for example only **review** + next **ready-for-dev** slice:

```bash
cd ~/dev_wsl/Vora/vora-website
# example: one review + four ready (tmux session required)
story-factory dispatch \
  7-9-smoke-test-deploy-auth-boundary \
  8-9-vitest-harness-token-state-tests \
  9-1-blog-list-page-isr \
  9-2-blog-detail-markdown-rendering \
  9-4-sitemap-dynamic-blog-urls \
  --parallel 4 --project-dir "$PWD"
```

Before that:

1. **`git status`** on **main** (primary clone): clean.  
2. **`gh auth status`** OK.  
3. Optional: **`story-factory cleanup`** so old merged worktrees do not confuse you.  
4. Confirm no other long `run` is holding the same worktree path.

---

## Biz-Brokerage / other repos

Same mechanics; worktrees live under **that** repo’s `.story-factory/worktrees/`. If dispatch was started from **`ai-biz-brokerage`** root, preconditions apply to **that** directory. Parallel product lines (vora vs biz) should use **separate tmux sessions** or sequential batches so preconditions and `gh` repo context stay obvious.

---

## Quick reference commands

```bash
# Unfinished stories (conceptually what argless dispatch uses): read YAML or use queue dry patterns
cd <repo>
story-factory queue --help   # batch sequential; dispatch is parallel tmux

# Single story with live output
story-factory run <story-key> --mode bmad --project-dir <worktree-or-repo> --verbose

# After merges
story-factory cleanup --project-dir <repo>
```

---

## Related repo docs

- Tool build/test: repo root `CLAUDE.md` (`just build`, `just test`, …).  
- High-level multi-project notes: `PROJECT_STATUS.md` (may be updated less often than sprint YAML).
