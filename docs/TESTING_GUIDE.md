# Story Factory — Testing & Usage Guide

This guide walks through setting up and running Story Factory against real projects.

## Prerequisites

### System Dependencies

| Dependency | Check | Notes |
|-----------|-------|-------|
| `claude` CLI | `which claude` | Claude Code CLI, must be on PATH |
| `bd` CLI (Gastown Beads) | `which bd` | Beads issue tracker, must be on PATH |
| `dolt` server | `which dolt` | Required backend for `bd` |
| Story Factory binary | `just build` | Produces `./story-factory` in repo root |

### Per-Project Requirements

Each target project needs:

1. **Sprint-status.yaml** at `_bmad-output/implementation-artifacts/sprint-status.yaml`
   - Must have `story_location: "{project-root}/_bmad-output/implementation-artifacts"` (the `{project-root}` placeholder is required)
   - Must have `development_status:` with stories in `backlog` status

2. **BMAD create-story skill** at `.claude/skills/bmad-create-story/`

3. **Beads database** initialized — a `.beads/` directory in the project root (see Beads Setup below)

---

## Installation

### Build and install the binary

```bash
# From the story-factory repo:
just build

# Option A: run with full path
./story-factory --help

# Option B: install to PATH
cp story-factory ~/go/bin/
# (or any directory in your PATH)
```

---

## Beads Setup (One-Time Per Project)

Story Factory's sync step calls `bd create` to create beads. Each project needs an initialized Beads database backed by a running Dolt server.

### 1. Start the Dolt SQL server (once per system)

```bash
# Start the shared Dolt server (runs on port 3307)
dolt sql-server --host 127.0.0.1 --port 3307 &

# Verify it's running
dolt sql-server --status 2>/dev/null || echo "Check: pgrep -a dolt"
```

> **Tip:** Add dolt sql-server to your shell startup or use a tmux session so it persists.

### 2. Initialize Beads in each project

```bash
# For vora-website:
cd ~/dev_wsl/Vora/vora-website
bd init --prefix vora --stealth
# --stealth keeps .beads/ out of git tracking
# --prefix sets the bead ID prefix (e.g., vora-1, vora-2)

# For Biz-Brokerage:
cd ~/dev_wsl/Biz-Brokerage/ai-biz-brokerage
bd init --prefix biz --stealth

# Verify each:
bd status
```

### 3. Test `bd create` manually

```bash
cd ~/dev_wsl/Vora/vora-website
bd create "test-bead: Smoke test" --notes "Verify bd works" --silent
# Should output a bead ID like: vora-abc123

# Clean up:
bd delete <bead-id>
```

---

## Project-Specific Fixes

### vora-website: Fix story_location path

The `story_location` field is missing the `{project-root}` placeholder. This will cause path resolution failures when using `--project-dir`.

```bash
# In ~/dev_wsl/Vora/vora-website/_bmad-output/implementation-artifacts/sprint-status.yaml
# Change:
#   story_location: _bmad-output/implementation-artifacts
# To:
#   story_location: "{project-root}/_bmad-output/implementation-artifacts"
```

### Biz-Brokerage: Note the nested project root

The actual project root is `~/dev_wsl/Biz-Brokerage/ai-biz-brokerage/` (not `~/dev_wsl/Biz-Brokerage/`). Always use the inner path with `--project-dir`.

---

## CLI Commands

### Single story — full pipeline (create → validate → sync)

```bash
story-factory run <story-key> --project-dir <path> [--verbose] [--dry-run]
```

This executes three steps in sequence:
1. **Create** — invokes Claude with the `create-story` BMAD skill to produce a story spec file
2. **Validate** — invokes Claude to validate the spec (up to 3 retry loops)
3. **Sync** — calls `bd create` with the story title and acceptance criteria, tags the file with `<!-- bead:ID -->`

### Single step commands

```bash
# Create only (no validate, no sync):
story-factory create-story <story-key> --project-dir <path>

# Validate only (story file must already exist):
story-factory validate-story <story-key> --project-dir <path>
```

### Batch — all backlog stories in an epic

```bash
story-factory epic <epic-number> --project-dir <path> [--verbose] [--dry-run]
```

Processes all `backlog` stories in the given epic sequentially. Re-reads sprint-status.yaml between stories for resumability.

### Batch — all backlog stories across all epics

```bash
story-factory queue --project-dir <path> [--verbose] [--dry-run]
```

Processes all backlog stories across all epics in order (epic 1 first, then 2, etc.).

### Global flags

| Flag | Purpose |
|------|---------|
| `--dry-run` | Show planned operations without executing subprocesses |
| `--verbose` | Stream Claude CLI output in real time |
| `--project-dir <path>` | Set project root (default: current working directory) |

---

## Testing Playbook

### Phase 1: Smoke Test (dry-run)

Verify preconditions pass and story-factory can read the sprint status.

```bash
# vora-website
story-factory epic 1 --project-dir ~/dev_wsl/Vora/vora-website --dry-run

# Biz-Brokerage
story-factory epic 1 --project-dir ~/dev_wsl/Biz-Brokerage/ai-biz-brokerage --dry-run
```

**Expected:** Lists the backlog stories it would process and the planned operations. Exit code 0.

**If precondition fails:** The error message tells you which check failed (bd CLI, sprint-status.yaml, or BMAD agents).

### Phase 2: Single Story Test (verbose)

Pick one backlog story from each project and run the full pipeline with verbose output.

```bash
# vora-website — first backlog story in epic 1
story-factory run 1-4-mobile-navigation-menu \
  --project-dir ~/dev_wsl/Vora/vora-website \
  --verbose

# Biz-Brokerage — first backlog story in epic 1
story-factory run 1-3-user-registration-flow-with-role-and-advisor-selection \
  --project-dir ~/dev_wsl/Biz-Brokerage/ai-biz-brokerage \
  --verbose
```

**What to watch for:**
- Create step: Claude should produce a story spec file in `_bmad-output/implementation-artifacts/`
- Validate step: May loop up to 3 times if validation finds issues
- Sync step: `bd create` should return a bead ID, and the story file should get a `<!-- bead:ID -->` comment appended

**Verify after:**
```bash
# Check story file was created
ls ~/dev_wsl/Vora/vora-website/_bmad-output/implementation-artifacts/1-4-*

# Check bead was created
cd ~/dev_wsl/Vora/vora-website && bd list --labels story
```

### Phase 3: Epic Batch Test

Once single stories work, run a full epic:

```bash
story-factory epic 1 \
  --project-dir ~/dev_wsl/Vora/vora-website \
  --verbose
```

**Expected:** Processes each backlog story in sequence. Already-processed stories (done/ready-for-dev) are skipped. Summary at end shows per-story results with bead IDs.

### Phase 4: Queue Test (optional)

Process all backlog stories across all epics:

```bash
story-factory queue \
  --project-dir ~/dev_wsl/Biz-Brokerage/ai-biz-brokerage \
  --verbose
```

---

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All stories succeeded (or no backlog stories) |
| 1 | At least one story failed |
| 2 | Precondition check failed (missing `bd`, missing sprint-status.yaml, missing BMAD skills) |

---

## Troubleshooting

### "bd CLI not found on PATH"
```bash
which bd  # Should show a path
# If missing, install Gastown Beads: https://github.com/steveyegge/gastown
```

### "no beads database found"
```bash
cd <project-root>
bd init --prefix <short-name> --stealth
```

### "sprint-status.yaml not found"
Check the path: `<project-root>/_bmad-output/implementation-artifacts/sprint-status.yaml`

### "BMAD agent files not found"
Check: `<project-root>/.claude/skills/bmad-create-story/` directory exists with SKILL.md and workflow files.

### "story_location field not found" or story files created in wrong place
Ensure `story_location` in sprint-status.yaml uses the `{project-root}` placeholder:
```yaml
story_location: "{project-root}/_bmad-output/implementation-artifacts"
```

### "bd create returned no bead ID"
- Check Dolt server is running: `pgrep -a dolt`
- Check bd works: `cd <project> && bd create "test" --silent`
- Story Factory expects bead IDs matching pattern `bd-*` or `<prefix>-*`

### Pipeline fails mid-batch
Story Factory is resumable. Just re-run the same command. It re-reads sprint-status.yaml between stories and skips already-processed ones.

---

## Project Status Summary

### vora-website (`~/dev_wsl/Vora/vora-website`)

| Check | Status | Action |
|-------|--------|--------|
| sprint-status.yaml | Exists | Fix `story_location` to add `{project-root}` prefix |
| BMAD skills | Installed | Ready |
| Backlog stories | 19 stories across 5 epics | Ready |
| Beads database | Not initialized | Run `bd init --prefix vora --stealth` |

### Biz-Brokerage (`~/dev_wsl/Biz-Brokerage/ai-biz-brokerage`)

| Check | Status | Action |
|-------|--------|--------|
| sprint-status.yaml | Exists | Ready (already has `{project-root}` placeholder) |
| BMAD skills | Installed | Ready |
| Backlog stories | 30+ stories across 11 epics | Ready |
| Beads database | Not initialized | Run `bd init --prefix biz --stealth` |

### Shared setup needed

| Check | Status | Action |
|-------|--------|--------|
| Dolt server | Not running | Start with `dolt sql-server` |
| story-factory on PATH | Built locally | Optionally `cp story-factory ~/go/bin/` |
