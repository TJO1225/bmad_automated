# Deferred Work

## Deferred from: code review of story 1-1-strip-legacy-code-and-rename-project (2026-04-06)

- `DefaultConfig()` in `internal/config/types.go` still defines 3 deleted workflows (`dev-story`, `code-review`, `git-commit`) and `FullCycle` steps — dead code, spec says don't modify config package in this story
- `internal/status/types.go` godoc references deleted `internal/router` package — Story 1.2 will rewrite the entire status package
- `CycleHeader()` in `internal/output/printer.go` hardcodes old "BMAD Full Cycle" branding and deleted pipeline steps — spec says don't modify output package in this story
- `README.md` and `CONTRIBUTING.md` reference old project name `bmad-automate` and deleted commands throughout — not in story 1.1 scope
- `docs/` directory (USER_GUIDE.md, CLI_REFERENCE.md, ARCHITECTURE.md, etc.) references old project name and deleted commands — not in story 1.1 scope
- `os.Stderr.WriteString` error return ignored in `internal/cli/root.go` — pre-existing, minor linter concern
