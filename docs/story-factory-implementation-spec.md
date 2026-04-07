# Story Factory — Implementation Spec

> **Purpose:** Hand this document to Claude Code on SP9 to rebuild the `bmad_automated` fork into a minimal Story Factory CLI.
>
> **Machine:** SP9 (Windows 11, WSL2, hostname Genie, user tjo12)
> **Repo:** `~/dev_wsl/clawsqwad/bmad_automated` (forked from robertguss/bmad_automated)
> **Language:** Go

---

## 1. What This Tool Does

Story Factory is a CLI that automates the repetitive part of BMAD sprint planning. After Tom manually completes BMAD planning through epic definition, this tool processes the backlog:

1. **Create Story (CS)** — Invokes `bmad-bmm-create-story` via `claude -p` for a backlog story. BMAD's Scrum Master (Bob) reads the PRD, architecture, and epics, then generates a full story file. BMAD also updates `sprint-status.yaml` automatically (`backlog → ready-for-dev`, epic `backlog → in-progress`).

2. **Validate Story (VS)** — Invokes the same `bmad-bmm-create-story` command again. BMAD detects the story file already exists and runs in validation mode (checklist verification). No separate command or prompt template needed.

3. **Sync to Beads** — Runs `bd create` to convert the validated story into a Gastown bead for implementation handoff.

Each step uses a fresh `claude -p` invocation (separate process = clean context). The tool re-reads `sprint-status.yaml` between stories since BMAD modifies it.

---

## 2. BMAD Input Contract

### What Exists Before the Tool Runs

Tom has already completed manual BMAD planning. The project repo contains:

```
{project-root}/
├── _bmad-output/
│   ├── implementation-artifacts/
│   │   └── sprint-status.yaml        # Manifest of all epics/stories with status
│   ├── planning-artifacts/
│   │   ├── prd.md                     # Product Requirements Document
│   │   ├── architecture.md            # Architecture document
│   │   ├── epics.md                   # All epic definitions with ACs
│   │   ├── ux-design-specification.md
│   │   └── ... (other planning docs)
│   └── test-artifacts/
├── project-context.md
└── ... (project source code)
```

### sprint-status.yaml Format

**CRITICAL: This is NOT the format the original fork expects.** The original fork parses nested `sprints[].stories[]` YAML. BMAD v6 uses a flat `development_status` map.

```yaml
generated: 2026-04-03
last_updated: 2026-04-03
project: ai-biz-brokerage
project_key: NOKEY
tracking_system: file-system
story_location: "{project-root}/_bmad-output/implementation-artifacts"

development_status:
  # Epic 1: Platform Foundation & User Authentication
  epic-1: backlog
  1-1-frontend-scaffold-and-biztender-design-system: backlog
  1-2-database-schema-and-user-authentication: backlog
  1-3-user-registration-flow-with-role-and-advisor-selection: backlog
  1-4-role-based-dashboard-navigation-and-empty-states: backlog
  epic-1-retrospective: optional

  # Epic 2: Seller Onboarding & Financial Intake
  epic-2: backlog
  2-1-ai-advisor-conversation-engine-and-onboarding-layout: backlog
  ...
```

**Key facts about this format:**
- Flat key-value map under `development_status`
- Epic entries: `epic-N: status`
- Story entries: `N-M-slug: status` (prefix digit = epic number)
- Retrospective entries: `epic-N-retrospective: optional|done`
- Comments with `#` provide epic titles (for display only)

**Status values (BMAD-native):**
| Status | Meaning |
|--------|---------|
| `backlog` | Story exists only in epics.md, no story file yet |
| `ready-for-dev` | Story file created by create-story, validated |
| `in-progress` | Developer actively working |
| `review` | Code review in progress |
| `done` | Story completed |
| `optional` | Retrospective entry (not a story) |

### Story File Output

`bmad-bmm-create-story` writes directly to:
```
_bmad-output/implementation-artifacts/{story-key}.md
```

Example: `_bmad-output/implementation-artifacts/1-1-frontend-scaffold-and-biztender-design-system.md`

No subfolder. The filename matches the key in `sprint-status.yaml` exactly.

### What BMAD Handles Automatically

When `bmad-bmm-create-story` runs via `claude -p`:
- Creates the story markdown file with full context (ACs, tasks, dev notes, architecture compliance, testing requirements)
- Updates `sprint-status.yaml`: story `backlog → ready-for-dev`
- If first story in epic: also updates epic `backlog → in-progress`
- Updates `last_updated` date

When run again on an existing story file:
- Switches to validation mode automatically
- Runs the story draft checklist
- Does NOT change status (story stays `ready-for-dev`)

**The Story Factory does NOT need to write to sprint-status.yaml.** It only reads it.

---

## 3. CLI Design

### Binary Name

`story-factory` (rename from `bmad-automate`)

### Commands

```
story-factory create-story <story-key>   # CS for one story
story-factory validate-story <story-key> # VS for one story (re-runs create-story on existing file)
story-factory sync-to-beads <story-key>  # Convert one validated story to a bead
story-factory run <story-key>            # Full pipeline: CS → VS → sync for one story
story-factory epic <epic-num>            # Full pipeline for all backlog stories in an epic
story-factory queue                      # Full pipeline for all backlog stories across all epics
```

### Flags

```
--project-dir <path>    # Project root (default: current directory)
--dry-run               # Show what would be done, don't execute
--verbose               # Show claude -p output in real time
```

### Processing Logic

**`create-story <story-key>`:**
1. Read `sprint-status.yaml`
2. Verify story exists and status is `backlog`
3. Run `claude -p` with prompt: `bmad-bmm-create-story {story-key}`
4. Wait for completion
5. Verify story file was created at expected path
6. Re-read `sprint-status.yaml` and verify status changed to `ready-for-dev`
7. Report success/failure

**`validate-story <story-key>`:**
1. Read `sprint-status.yaml`
2. Verify story exists and status is `ready-for-dev`
3. Verify story file exists at expected path
4. Run `claude -p` with prompt: `bmad-bmm-create-story {story-key}` (same command — BMAD auto-detects validation mode)
5. Wait for completion
6. Report validation result

**`sync-to-beads <story-key>`:**
1. Read `sprint-status.yaml`
2. Verify story status is `ready-for-dev` (validated)
3. Read story file to extract title and acceptance criteria
4. Run `bd create "{story-key}: {title}" --notes "{acceptance-criteria}"`
5. Parse bead ID from `bd create` output
6. Append `<!-- bead:{bead-id} -->` to story file for bidirectional tracking
7. Report bead ID

**`run <story-key>`:**
1. Run `create-story`
2. If success → run `validate-story`
3. If success → run `sync-to-beads`
4. Clear context between each step (each is a separate `claude -p` call, so this is automatic)

**`epic <epic-num>`:**
1. Read `sprint-status.yaml`
2. Find all stories with key prefix `{epic-num}-` and status `backlog`
3. For each story (in key order): run the full `run` pipeline
4. Re-read YAML between stories (BMAD updates it after each create-story)
5. Report summary: N stories created, N validated, N beads created

**`queue`:**
1. Read `sprint-status.yaml`
2. Find all stories with status `backlog` across all epics
3. Group by epic, process in epic order then story order
4. For each story: run the full `run` pipeline
5. Re-read YAML between stories
6. Report summary

---

## 4. Sprint Status Parser — REWRITE

The original `internal/status/` package must be **completely rewritten**. It currently parses nested `sprints[].stories[]` YAML. We need a parser for the flat `development_status` map.

### Data Structures

```go
type SprintStatus struct {
    Generated   string                 `yaml:"generated"`
    LastUpdated string                 `yaml:"last_updated"`
    Project     string                 `yaml:"project"`
    ProjectKey  string                 `yaml:"project_key"`
    TrackingSystem string              `yaml:"tracking_system"`
    StoryLocation  string              `yaml:"story_location"`
    DevStatus   map[string]string      `yaml:"development_status"`
}

type StoryEntry struct {
    Key      string // e.g. "1-1-frontend-scaffold-and-biztender-design-system"
    EpicNum  int    // parsed from key prefix
    StoryNum int    // parsed from key (second number)
    Status   string // "backlog", "ready-for-dev", etc.
}

type EpicEntry struct {
    Num    int
    Key    string // "epic-1"
    Status string
}
```

### Parser Requirements

- Read `_bmad-output/implementation-artifacts/sprint-status.yaml`
- Parse the flat `development_status` map
- Classify each entry as epic (`epic-N`), story (`N-M-...`), or retrospective (`epic-N-retrospective`)
- Extract epic number from story key prefix (first digit before first `-`)
- Sort stories by epic number, then story number
- Filter by status (e.g., "give me all backlog stories in epic 1")

---

## 5. Claude Client — KEEP AND SIMPLIFY

The original `internal/claude/` package runs `claude -p "prompt" --output-format stream-json` and parses streaming JSON output. **Keep this.** It's the engine.

### Simplification

- Remove any workflow-specific logic (the client should be a generic "run prompt, return result" function)
- Keep streaming JSON parser
- Keep error detection
- Keep timeout handling
- The prompt for both CS and VS is identical: `bmad-bmm-create-story {story-key}`

### Prompt Template

Only one prompt is needed (in `config/default.yaml` or hardcoded):

```
bmad-bmm-create-story {story-key}
```

That's it. BMAD's own agent files handle all the context loading, story generation, and validation detection internally. The Story Factory doesn't need to construct complex prompts.

---

## 6. Beads Package — NEW

Create `internal/beads/` to handle the `bd create` integration.

### Requirements

- Shell out to `bd create` (assumes `bd` is on PATH)
- Parse the bead ID from stdout
- Append tracking comment to story file
- Handle errors (bd not installed, bd not initialized in repo)

### Story File Parsing

Extract from the story markdown file:
- **Title:** First `# Story X.Y: Title` heading
- **Acceptance Criteria:** Content between `## Acceptance Criteria` and the next `## ` heading

### bd create Invocation

```bash
bd create "{story-key}: {extracted-title}" --notes "{acceptance-criteria-text}"
```

Keep it simple. No `--blocks` dependency chaining for v1 — that can be added later.

---

## 7. What to Remove from the Fork

**Delete entirely:**
- `internal/cli/dev_story.go` (and any dev-story command registration)
- `internal/cli/code_review.go` (and any code-review command registration)
- `internal/cli/git_commit.go` (and any git-commit command registration)
- `internal/cli/full_cycle.go` or any "run all four workflows" command
- Any prompt templates in `config/default.yaml` for dev-story, code-review, git-commit
- Any test files for removed commands

**Rewrite completely:**
- `internal/status/` — New flat YAML parser (see Section 4)
- `internal/router/` — Simplified to only our three transitions (or remove entirely if routing is handled in CLI command logic)
- `internal/state/` — Simplified state machine (or remove if not needed)
- `internal/lifecycle/` — Replace with our CS → VS → sync pipeline
- `internal/cli/` — Three new commands + epic + queue + run
- `config/default.yaml` — Single prompt template
- `CLAUDE.md` — Updated for new tool
- `README.md` — Updated for new tool

**Keep and simplify:**
- `internal/claude/` — Generic Claude pipe client
- `internal/config/` — Viper config loading
- `internal/output/` — Terminal styling
- `cmd/bmad-automate/main.go` — Rename to `cmd/story-factory/` or update binary name
- `go.mod` / `go.sum` — Update module name
- `justfile` — Update recipes

**Add new:**
- `internal/beads/` — bd create integration (see Section 6)

---

## 8. File-by-File Action Plan

Read the codebase first (`find internal/ -name "*.go"`), then execute in this order:

### Phase 1: Strip and Compile
1. Delete all removed command files
2. Remove their registration from CLI root command
3. Remove unused prompt templates from config
4. Rename binary/module if desired
5. **Verify: `go build` succeeds**

### Phase 2: Rewrite Status Parser
1. Rewrite `internal/status/` for flat `development_status` YAML
2. Add helper methods: `BacklogStories()`, `StoriesForEpic(n)`, `StoryByKey(key)`
3. Write tests against the sample YAML in Section 2
4. **Verify: tests pass**

### Phase 3: Build Core Commands
1. Implement `create-story` command (invoke claude -p, verify result)
2. Implement `validate-story` command (same invocation, check existing file)
3. Create `internal/beads/` package
4. Implement `sync-to-beads` command
5. **Verify: each command works individually on a test project**

### Phase 4: Pipeline Commands
1. Implement `run` (CS → VS → sync for one story)
2. Implement `epic` (all backlog stories in one epic)
3. Implement `queue` (all backlog stories across all epics)
4. Add `--dry-run` support throughout
5. **Verify: `story-factory epic 1` processes all Epic 1 backlog stories end-to-end**

### Phase 5: Polish
1. Update `CLAUDE.md`
2. Update `README.md`
3. Update `justfile`
4. Clean up any dead imports or unused packages
5. Tag `v0.1.0`

---

## 9. Working Style for Claude Code

- **Read before modifying.** Before changing any file, `cat` it first to understand the existing patterns.
- **Kaizen.** Build and verify each phase before moving to the next. Run `go build` after every structural change.
- **Copy-pasteable.** When showing commands to run, make them ready to paste.
- **Test at each step.** After every modification, verify the build compiles and any existing tests still pass.
- **Minimize.** When in doubt, remove code rather than keep it. The goal is the thinnest possible tool that does exactly CS → VS → sync.

---

## 10. Prerequisites (Verify Before Starting)

```bash
# Go installed
go version

# Claude Code CLI available
claude --version

# Beads CLI available
bd --version

# Fork cloned
cd ~/dev_wsl/clawsqwad/bmad_automated
git remote -v

# BMAD installed (check for agent files)
ls ~/.claude/skills/bmad/agents/ 2>/dev/null || ls .bmad-core/ 2>/dev/null

# Sample project with sprint-status.yaml exists
ls {project-root}/_bmad-output/implementation-artifacts/sprint-status.yaml
```

---

## 11. Sample sprint-status.yaml for Testing

Use this for parser development and testing:

```yaml
generated: 2026-04-03
last_updated: 2026-04-03
project: ai-biz-brokerage
project_key: NOKEY
tracking_system: file-system
story_location: "{project-root}/_bmad-output/implementation-artifacts"

development_status:
  epic-1: backlog
  1-1-frontend-scaffold-and-biztender-design-system: backlog
  1-2-database-schema-and-user-authentication: backlog
  1-3-user-registration-flow-with-role-and-advisor-selection: backlog
  1-4-role-based-dashboard-navigation-and-empty-states: backlog
  epic-1-retrospective: optional

  epic-2: backlog
  2-1-ai-advisor-conversation-engine-and-onboarding-layout: backlog
  2-2-elevenlabs-voice-integration: backlog
  epic-2-retrospective: optional
```
