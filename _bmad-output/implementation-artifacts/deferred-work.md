# Deferred Work

## Deferred from: code review of story 1-1-strip-legacy-code-and-rename-project (2026-04-06)

- `DefaultConfig()` in `internal/config/types.go` still defines 3 deleted workflows (`dev-story`, `code-review`, `git-commit`) and `FullCycle` steps — dead code, spec says don't modify config package in this story
- `internal/status/types.go` godoc references deleted `internal/router` package — Story 1.2 will rewrite the entire status package
- `CycleHeader()` in `internal/output/printer.go` hardcodes old "BMAD Full Cycle" branding and deleted pipeline steps — spec says don't modify output package in this story
- `README.md` and `CONTRIBUTING.md` reference old project name `bmad-automate` and deleted commands throughout — not in story 1.1 scope
- `docs/` directory (USER_GUIDE.md, CLI_REFERENCE.md, ARCHITECTURE.md, etc.) references old project name and deleted commands — not in story 1.1 scope
- `os.Stderr.WriteString` error return ignored in `internal/cli/root.go` — pre-existing, minor linter concern

## Deferred from: code review of 2-2-story-creation-step.md (2026-04-06)

- No automated test drives `DefaultExecutor` through context cancel with `GracePeriod` (SIGTERM, `WaitDelay`, then kill); only configuration fields are asserted. Add an integration-style test when a safe subprocess stub is available.

## Deferred from: code review of 2-5-full-pipeline-composition-and-run-command.md (2026-04-06)

- Story Task 4 testing strategy preferred driving `Run()` through `claude.MockExecutor` / `beads.MockExecutor`; implementation tests orchestration via `runPipeline` with injected `stepFunc` instead. Acceptable tradeoff for sequencing coverage; add executor-level integration tests later if desired.

## Deferred from: code review of 2-4-beads-synchronization.md (2026-04-06)

- `ParseBeadID` assumes the first non-empty line of `bd create` stdout is the bead ID; fragile if `bd` adds preamble or logging — verify against real CLI output or document the contract [internal/beads/executor.go:72]

## Deferred from: code review of story 1-3-sprint-status-queries-and-path-resolution (2026-04-06)

- Non-scalar development_status values silently produce empty Status — `Read()` accesses `.Value` on value nodes without checking `Kind == ScalarNode`; a nested mapping value would produce `Status("")` silently [internal/status/reader.go:100]
- Unclassified keys default to EntryTypeStory with zero epic/story numbers — any key not matching the three regexes (typo, new format) is silently treated as a story with EpicNum=0, StoryNum=0; no way for callers to distinguish from a real entry [internal/status/reader.go:259-261]

## Deferred from: code review of 3-1-epic-batch-command.md (2026-04-06)

- `RunQueue` shipped in `internal/pipeline/batch.go` before Story 3-2; Story 3-1 text said not to add it yet, but `queue` CLI already calls `RunQueue` — explicitly accept or relocate in Story 3-2 scope rather than debating as a 3-1-only defect
