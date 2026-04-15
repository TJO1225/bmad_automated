# User Guide

Practical recipes for using `story-factory` on a BMAD v6 project.

For a command-by-command reference, see [CLI_REFERENCE.md](CLI_REFERENCE.md).

## 1. First-time setup

```bash
# Build the binary
git clone <repo> && cd bmad_automated
just build

# Or install globally so it's on PATH
go install ./cmd/story-factory
```

Verify:

```bash
story-factory --help
```

Preflight any BMAD project you plan to use it against:

```bash
cd ~/your-project
ls _bmad-output/implementation-artifacts/sprint-status.yaml
ls .claude/skills/bmad-{create-story,dev-story,code-review}/
which gh          # bmad mode needs this
gh auth status    # must be authenticated for open-pr to work
```

## 2. The two modes

story-factory supports two pipelines. Pick one per invocation with `--mode`.

### bmad mode (default)

Full loop from backlog to PR. Use for day-to-day development.

```
create-story → dev-story → code-review → commit-branch → open-pr
```

At the end you have a `story/<key>` branch pushed to origin and a PR
awaiting review on GitHub.

### beads mode

Create the story and hand off to Gastown Beads. Use when your project
tracks implementation separately in Beads.

```
create-story → sync-to-beads
```

Invoke with `--mode=beads`.

## 3. Common workflows

### Run one story end-to-end

```bash
cd ~/your-project
story-factory run 1-2-database-schema
```

Takes you from backlog to a PR against main. Watch it stream with `--verbose`.

### Run a whole epic sequentially

```bash
story-factory epic 1
```

Stories run one at a time in order. Total time ≈ sum of individual story
times. Good for epics where stories build on each other.

### Drain the backlog sequentially

```bash
story-factory queue
```

All backlog stories across all epics, in order. Same sequential behavior
as `epic`.

### Run many stories in parallel (tmux)

```bash
# from inside a tmux session
cd ~/your-project
story-factory dispatch --parallel 4
```

Creates 4 panes (one window, 2×2 tiled) and runs 4 stories concurrently,
each in its own git worktree under `.story-factory/worktrees/`. As each
story finishes, its pane picks up the next story in the queue.

Cap at whatever you can monitor — 4 is typical, 8 (two windows) if you
have a large screen.

Specific keys only:

```bash
story-factory dispatch 1-2 1-3 1-5 --parallel 3
```

### Preview before executing

```bash
story-factory run 1-2-database-schema --dry-run
story-factory dispatch --parallel 4 --dry-run
```

No Claude calls, no git ops. Dry-run confirms preconditions pass and
shows the planned sequence.

### Clean up merged worktrees

After PRs merge:

```bash
story-factory cleanup
```

Removes any worktree under `.story-factory/worktrees/` whose
`story/<key>` branch is now an ancestor of the default branch.

## 4. Recovering from interruptions

story-factory is resume-safe. Typical recovery patterns:

### A single story crashed mid-run

```bash
# Check current status:
grep '1-2-database-schema' _bmad-output/implementation-artifacts/sprint-status.yaml
# development_status:
#   1-2-database-schema: in-progress         # dev-story was partway

# Just re-run it:
story-factory run 1-2-database-schema
```

- `create-story` → skipped (status past `backlog`)
- `dev-story` → runs; BMAD's own skill resumes from its partial state
- `code-review` → runs if dev-story finishes
- `commit-branch` / `open-pr` → run

### Dispatch crashed partway through an epic

```bash
# Some worktrees exist, others don't. Just re-run:
story-factory dispatch --parallel 4
```

For each story the dispatcher will:

- reuse an existing worktree at `.story-factory/worktrees/<key>` if present
- create a new one otherwise

Inside each worktree, `story-factory run` applies the per-story resume
logic above. Stories already at `done` are skipped entirely.

### `gh pr create` failed after a successful commit

Most common cause: you weren't logged in to `gh`. Fix and re-run:

```bash
gh auth login
story-factory run 1-2-database-schema
```

`open-pr` detects the pre-existing push and either re-pushes (no-op) and
creates the PR, or — if a prior run already created one — finds the
existing PR via `gh pr view` and records its URL.

### code-review found issues (status flipped back to `in-progress`)

story-factory stops with `needs-review` in the summary. Address the
findings in the story file's review section, then re-run:

```bash
# Story status is now in-progress, but dev-story resume will accept that.
story-factory run 1-2-database-schema
```

If the review pass was pure polish and you want to force-complete,
manually bump the status to `done` in `sprint-status.yaml` and skip to
commit-branch / open-pr.

### Worktree got into a weird state

```bash
# Nuclear option — delete the worktree + branch and restart:
git worktree remove .story-factory/worktrees/1-2-foo --force
git branch -D story/1-2-foo
story-factory run 1-2-foo
```

## 5. Tips

**Watch a dispatcher run**: the dispatcher's own output goes to the pane
you invoked it from. The panes it spawns show each story's live output.
Use Ctrl-b q to flash pane numbers so you can jump around.

**Test on one story first** before dispatching a whole epic. The first
run always uncovers some environmental surprise.

**Keep `sprint-status.yaml` under version control.** It's the source of
truth for resume. If you lose it, story-factory has no way to know what
ran.

**Don't edit `sprint-status.yaml` by hand while a pipeline is running** —
BMAD's skills write to it too, and simultaneous edits will race.

## 6. When not to use this

- One-off experiments where BMAD's structure is overkill
- Projects without BMAD v6 installed
- Stories that don't map to a single PR (e.g., massive multi-week epics
  that span many commits). Break those down first with `bmad-create-story`
  manually.

## Related docs

- [CLI_REFERENCE.md](CLI_REFERENCE.md) — every flag
- [ARCHITECTURE.md](ARCHITECTURE.md) — internals
- [BMAD v6 docs](https://docs.bmad-method.org/) — the skills story-factory drives
