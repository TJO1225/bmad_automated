# story-factory

A Go CLI that orchestrates [BMAD v6](https://docs.bmad-method.org/) skills via
`claude -p` to turn a sprint backlog into merged pull requests — one story at
a time or many in parallel across tmux panes.

## What it does

Given a BMAD v6 project with a populated `sprint-status.yaml`, story-factory
drives each backlog story through a configurable pipeline of BMAD skills,
updating sprint status and producing a PR per story.

Two modes:

- **`bmad`** (default) — `create-story → dev-story → code-review → commit-branch → open-pr`.
  All five steps for each story. Each Claude step is a fresh `claude -p`
  subprocess so context doesn't bleed between steps. Ends with a PR on a
  `story/<key>` branch.
- **`beads`** — `create-story → sync-to-beads`. Creates the story file and
  hands it off to [Gastown Beads](https://github.com/gastown/beads) via
  `bd create`. Use when you track work in Beads rather than PRs.

Both modes can be driven sequentially (one story at a time) or parallelized
across tmux panes with per-story git worktrees.

## Requirements

- Go 1.21+
- [Claude CLI](https://github.com/anthropics/claude-code) on PATH
- BMAD v6 installed in the project (`.claude/skills/bmad-create-story/`,
  `.claude/skills/bmad-dev-story/`, `.claude/skills/bmad-code-review/`)
- `gh` CLI on PATH (bmad mode, for opening PRs)
- `bd` CLI on PATH (beads mode only)
- `tmux` (dispatch command only)
- `just` (optional, for build tasks)

## Install

```bash
git clone <this repo>
cd bmad_automated
just build           # produces ./story-factory
# or:
go install ./cmd/story-factory   # installs to $GOPATH/bin
```

## Project layout expected by story-factory

Run story-factory from the root of a BMAD v6 project that already has:

```
<project-root>/
├── _bmad-output/
│   ├── implementation-artifacts/
│   │   └── sprint-status.yaml          # flat dev_status map, BMAD v6 shape
│   └── planning-artifacts/
│       ├── prd.md
│       ├── architecture.md
│       └── epics.md
├── .claude/skills/
│   ├── bmad-create-story/
│   ├── bmad-dev-story/
│   └── bmad-code-review/
└── (your project source)
```

story-factory reads the flat `development_status` map in sprint-status.yaml
and never writes it — BMAD's skills mutate sprint-status as a side effect of
running.

## Commands

All commands accept these global flags:

| Flag | Default | Purpose |
|------|---------|---------|
| `--mode` | `bmad` | Pipeline shape: `bmad` or `beads` |
| `--project-dir` | cwd | Override project root |
| `--dry-run` | off | Print what would run without executing subprocesses |
| `--verbose` | off | Stream each Claude subprocess's output |

### create-story

Create a single story file from a backlog entry.

```bash
story-factory create-story 1-2-database-schema
```

Runs only the `/bmad-create-story` step. Use when you want a draft but don't
want to continue through dev/review yet.

### run

Run the full pipeline for one story.

```bash
story-factory run 1-2-database-schema --mode=bmad
```

**Resume-safe**: if the story's status is already past a step (e.g.
`review`), that step is skipped without re-invoking Claude. Re-running a
partially-completed story picks up where the previous run left off.

### epic

Run the full pipeline for every **unfinished** story in one epic
(anything not yet `done`), sequentially.

```bash
story-factory epic 1 --mode=bmad
```

Finds stories keyed `1-*-*`, filters out ones already at `done`, sorts by
story number, runs each to completion. Stories at `ready-for-dev`,
`in-progress`, or `review` flow into `run`, which applies per-step resume
logic so work already done isn't repeated.

### queue

Run the full pipeline for every unfinished story across all epics,
sequentially, in epic-then-story order.

```bash
story-factory queue --mode=bmad
```

### dispatch — parallel across tmux panes

Run multiple stories in parallel. Must be invoked from inside a tmux
session. Each story gets its own git worktree and its own tmux pane.

```bash
# from inside tmux, in a BMAD project with clean main:
story-factory dispatch 1-1 1-2 1-3 1-4 1-5 1-6 --parallel 4

# or pull all backlog stories:
story-factory dispatch --parallel 4
```

Layout: `ceil(N/4)` new tmux windows named `sf-batch-<n>`, each with a 2×2
tiled layout (or fewer panes if there aren't enough stories). Each slot
loops: pull next story → create worktree → run `story-factory run <key>
--mode=bmad --project-dir <worktree>` → scrape sentinel from pane output →
pick up the next story in the same pane.

Worktrees live at `.story-factory/worktrees/<key>` until you run
`story-factory cleanup`.

### cleanup

Remove worktrees whose `story/<key>` branch has been merged into the
default branch.

```bash
story-factory cleanup
story-factory cleanup --force   # remove even if not merged
```

## The pipeline in detail

For `--mode=bmad`, each story runs through five steps:

1. **`create-story`** — invokes `/bmad-create-story <key>` via Claude.
   BMAD drafts the story markdown at
   `_bmad-output/implementation-artifacts/<key>.md` and flips sprint status
   `backlog → ready-for-dev`.
2. **`dev-story`** — invokes `/bmad-dev-story <key>`. BMAD implements the
   story in the working tree (no git commits). Sprint status transitions
   `ready-for-dev → in-progress → review`.
3. **`code-review`** — invokes `/bmad-code-review <key>`. BMAD reviews
   uncommitted changes. On a clean review, status flips `review → done`.
   If findings remain, BMAD flips it back to `in-progress` — the pipeline
   stops with `needs-review` and you resolve manually, then re-run.
4. **`commit-branch`** — native git (no Claude). Creates `story/<key>`
   branch, stages all changes, commits with message
   `feat(<key>): <title>` plus the story's acceptance criteria. Skips the
   branch-create step if you're already on `story/<key>` (the dispatcher
   case).
5. **`open-pr`** — native `git push -u origin` + `gh pr create --base <default>`.
   Parses the PR URL out of gh's output and surfaces it in the summary.
   If a PR already exists for the branch (resume after partial failure),
   skips creation and reuses the existing URL.

For `--mode=beads`, steps 2–5 collapse to a single `sync-to-beads` step
that runs `bd create` and appends a `<!-- bead:<id> -->` tracking comment
to the story file.

## Resuming interrupted runs

story-factory is resume-safe at two levels:

**Per-story (in `story-factory run <key>`)**: each step checks sprint status
before invoking Claude. Mapping:

| Sprint status | `create-story` | `dev-story` | `code-review` |
|---|---|---|---|
| `backlog` | runs | fails¹ | fails¹ |
| `ready-for-dev` | skip | runs | fails¹ |
| `in-progress` | skip | runs (BMAD resumes) | fails¹ |
| `review` | skip | skip | runs |
| `done` | skip | skip | skip |

¹ Fails cleanly with an actionable reason, not silently.

Top-level gate: if a story is already `done`, the whole pipeline short-
circuits.

**Per-batch (in `dispatch`)**: if
`.story-factory/worktrees/<key>` already exists, the dispatcher reuses the
worktree instead of trying to `git worktree add` again. Combined with the
per-story resume above, you can safely re-run `dispatch` after a crash and
stories will pick up from wherever they left off.

If `open-pr` already pushed and opened a PR in a prior run, `open-pr`
detects this via `gh pr view <branch>` and records the existing URL
instead of failing.

## Preconditions

Each run validates its environment before touching anything:

- `sprint-status.yaml` exists at `_bmad-output/implementation-artifacts/`
- Required BMAD skill dirs exist under `.claude/skills/` (mode-dependent)
- `bd` on PATH — **beads mode only**
- `gh` on PATH — **bmad mode only**
- Working tree is a clean git repo — **bmad mode only**

Precondition failure exits with code **2**. Story failure exits **1**.
Success exits **0**.

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | All stories succeeded (or no work to do) |
| 1 | At least one story failed |
| 2 | Precondition check failed |

## Configuration

Defaults are built-in (`internal/config/types.go`). You can override via
`config/workflows.yaml`:

```yaml
workflows:
  create-story:
    prompt_template: "/bmad-create-story - Create story: {{.StoryKey}}. Do not ask questions."
  dev-story:
    prompt_template: "/bmad-dev-story - Implement story: {{.StoryKey}}. Do not ask questions."
  code-review:
    prompt_template: "/bmad-code-review - Review story: {{.StoryKey}}. Review uncommitted changes in the working tree."

modes:
  bmad:
    steps: [create-story, dev-story, code-review, commit-branch, open-pr]
  beads:
    steps: [create-story, sync-to-beads]

claude:
  output_format: stream-json
  binary_path: claude
```

Env overrides (Viper-style): `BMAD_CLAUDE_PATH`, `BMAD_CONFIG_PATH`.

## Development

```bash
just build         # compile ./story-factory
just test          # run tests
just check         # fmt + vet + test
just test-pkg ./internal/pipeline   # single package
```

Key packages:

| Package | Responsibility |
|---------|----------------|
| `internal/cli` | Cobra commands, app wiring, exit-code plumbing |
| `internal/pipeline` | Step definitions, pipeline orchestration, batch runners |
| `internal/config` | Viper config + mode/workflow definitions |
| `internal/claude` | Claude subprocess executor + streaming JSON parser |
| `internal/status` | sprint-status.yaml reader |
| `internal/storyfile` | Title / AC extraction from story markdown |
| `internal/beads` | `bd create` wrapper (beads mode only) |
| `internal/git` | Thin `git` CLI wrappers (branches, worktrees) |
| `internal/tmux` | Thin `tmux` CLI wrappers + parallel dispatcher |
| `internal/output` | Terminal printer (structured summaries) |

See `docs/ARCHITECTURE.md` for the deeper dive.

## License

MIT — see [LICENSE](LICENSE).
