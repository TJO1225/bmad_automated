# CLI Reference

Complete command and flag reference for `story-factory`.

## Synopsis

```
story-factory [command] [arguments] [flags]
```

## Global flags

Available on every subcommand.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--mode` | string | `bmad` | Pipeline mode: `bmad` or `beads` |
| `--project-dir` | string | current working directory | Path to BMAD project root |
| `--dry-run` | bool | `false` | Show planned operations without executing subprocesses |
| `--verbose` | bool | `false` | Stream Claude subprocess output in real time |
| `-h`, `--help` | — | — | Help for any command |

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success, or no work needed |
| 1 | At least one story failed during processing |
| 2 | Precondition check failed (missing BMAD skill, bd/gh CLI, dirty tree, etc.) |

## Commands

### `create-story <story-key>`

Invoke `/bmad-create-story <key>` once. Does not continue into dev/review.

```bash
story-factory create-story 1-2-database-schema
```

Succeeds when:
- The story file exists at `_bmad-output/implementation-artifacts/<key>.md`
- Sprint status has advanced from `backlog`

Fails (exit 1) on Claude subprocess error or post-condition miss.

### `run <story-key>`

Run the full mode-dependent pipeline for one story.

```bash
story-factory run 1-2-database-schema               # bmad mode
story-factory run 1-2-database-schema --mode=beads  # beads mode
```

**Resume behavior**: each step checks sprint status first and skips if the
story has already progressed past it.

| Starting status | Steps that run |
|---|---|
| `backlog` | all |
| `ready-for-dev` | dev-story, code-review, commit-branch, open-pr |
| `in-progress` | dev-story (resumes), code-review, commit-branch, open-pr |
| `review` | code-review, commit-branch, open-pr |
| `done` | none (whole pipeline short-circuits at top level) |

Beads mode: same pattern but only `create-story` and `sync-to-beads`.

### `epic <epic-number>`

Sequential run for every **unfinished** story in one epic.

```bash
story-factory epic 1
```

- Lists stories keyed `<epic-number>-*-*`, filters to non-done only, sorts
  by story number.
- Runs each through `run` to completion before advancing. `run` applies
  per-step resume logic, so stories already at `ready-for-dev`,
  `in-progress`, or `review` pick up where they left off.
- Single-story failure counts as batch failure but does not stop the batch.

### `queue`

Same as `epic` but across all epics, ordered by epic then story. Processes
every story whose status is not `done`.

```bash
story-factory queue
```

### `dispatch [story-keys...] [--parallel N]`

Parallel execution across tmux panes with per-story git worktrees.

```bash
# all unfinished stories (anything not at done), 4 in parallel
story-factory dispatch --parallel 4

# specific keys (bypasses the unfinished filter — any key is accepted)
story-factory dispatch 1-2 1-3 2-1 --parallel 3
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `-p`, `--parallel` | 4 | Max concurrent stories |

Behavior:

- Requires `$TMUX` to be set (must be invoked from inside tmux)
- Creates `ceil(parallel / 4)` new tmux windows named `sf-batch-<n>`,
  each with a 2×2 tiled pane layout (fewer panes if parallel % 4 ≠ 0)
- For each story: `git worktree add -b story/<key> .story-factory/worktrees/<key> <base>`,
  then `tmux send-keys` to run `story-factory run <key> --mode=bmad
  --project-dir <wt>`; watches the pane for a nonce-tagged sentinel to
  detect completion
- **Resume**: if `.story-factory/worktrees/<key>` already exists, reuse it
  instead of creating a new worktree
- Panes are reused — when a story finishes, the next queued story runs
  in the same pane

Exit codes inherit from the aggregated batch result.

### `cleanup [--force]`

Remove worktrees whose `story/<key>` branch has been merged into the
default branch.

```bash
story-factory cleanup
story-factory cleanup --force   # even if not merged
```

Flags:

| Flag | Default | Description |
|------|---------|-------------|
| `--force` | off | Remove worktrees regardless of merge status (also uses `git branch -D`) |

Only touches worktrees under `.story-factory/worktrees/` on `story/*`
branches — worktrees you created by hand are ignored.

## Mode reference

### `bmad` mode

Default. Five steps per story:

1. `create-story` — `/bmad-create-story <key>` via Claude
2. `dev-story` — `/bmad-dev-story <key>` via Claude
3. `code-review` — `/bmad-code-review <key>` via Claude
4. `commit-branch` — native git: create `story/<key>`, stage all, commit
5. `open-pr` — native `git push -u origin` + `gh pr create`

Preconditions: sprint-status.yaml + all three BMAD skill dirs + `gh` CLI +
clean git tree.

### `beads` mode

Two steps per story:

1. `create-story` — `/bmad-create-story <key>` via Claude
2. `sync-to-beads` — `bd create` with the story title/ACs, appends
   `<!-- bead:<id> -->` tracking comment to the story file

Preconditions: sprint-status.yaml + `bmad-create-story` skill + `bd` CLI.
No gh, no clean-tree requirement.

## Configuration

Built-in defaults (see `internal/config/types.go`) are used if no config
file is present. Override via `config/workflows.yaml`:

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

output:
  truncate_lines: 20
  truncate_length: 60
```

Environment variables (Viper prefix `BMAD_`):

| Variable | Description |
|----------|-------------|
| `BMAD_CONFIG_PATH` | Explicit path to a config YAML |
| `BMAD_CLAUDE_PATH` | Override `claude` binary path |

## File locations

| Path | Purpose |
|------|---------|
| `_bmad-output/implementation-artifacts/sprint-status.yaml` | Source of truth for story state |
| `_bmad-output/implementation-artifacts/<key>.md` | Per-story spec file (written by `create-story`) |
| `.claude/skills/bmad-{create-story,dev-story,code-review}/` | BMAD v6 skill directories |
| `.story-factory/worktrees/<key>/` | Dispatcher-managed worktree per story |
| `config/workflows.yaml` | Optional config override |
