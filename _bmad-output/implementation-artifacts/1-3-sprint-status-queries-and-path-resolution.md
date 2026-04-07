# Story 1.3: Sprint Status Queries and Path Resolution

Status: review

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an operator,
I want to look up stories by status and resolve story file paths from sprint-status.yaml metadata,
so that the pipeline can determine which stories need processing and where their files live.

## Acceptance Criteria

1. **Given** a parsed sprint status with stories in various statuses
   **When** I call `StoriesByStatus("backlog")`
   **Then** only story entries (not epics or retrospectives) with `backlog` status are returned
   **And** entries are sorted by epic number then story number (numeric, not lexicographic)

2. **Given** a parsed sprint status with no stories matching the requested status
   **When** I call `StoriesByStatus("review")`
   **Then** an empty slice is returned (not nil, not an error)

3. **Given** a sprint-status.yaml with `story_location: "{project-root}/_bmad-output/implementation-artifacts"`
   **When** I call `ResolveStoryLocation("/home/tom/projects/my-app")`
   **Then** the resolved path is `/home/tom/projects/my-app/_bmad-output/implementation-artifacts`

4. **Given** a sprint-status.yaml WITHOUT a `story_location` field
   **When** I call `ResolveStoryLocation("/home/tom/projects/my-app")`
   **Then** a clear error is returned indicating the field is missing

5. **Given** an empty projectDir string passed to `ResolveStoryLocation`
   **When** the method resolves the path
   **Then** `{project-root}` is replaced with empty string (the caller decides CWD semantics)

## Tasks / Subtasks

- [x] Task 1: Add `StoriesByStatus()` method to `internal/status/reader.go` (AC: #1, #2)
  - [x] Add `StoriesByStatus(status string) ([]Entry, error)` — calls `Read()`, filters to story entries matching the given status string, sorted by epic number then story number
  - [x] Accept any status string (not just valid ones) since the parser is lenient about status values (per Story 1.2 design: retrospectives use `optional`)
  - [x] Return empty slice (not nil) when no entries match
- [x] Task 2: Add metadata reading to `internal/status/reader.go` (AC: #3, #4, #5)
  - [x] Add internal method `readStoryLocation() (string, error)` that reads the YAML file and extracts the `story_location` top-level field using Node API
  - [x] Return a clear error if `story_location` key is not found in the root mapping
  - [x] Re-read from disk (no caching) — consistent with all other Reader methods
- [x] Task 3: Add `ResolveStoryLocation()` method to `internal/status/reader.go` (AC: #3, #4, #5)
  - [x] Add `ResolveStoryLocation(projectDir string) (string, error)` — reads story_location and replaces `{project-root}` with projectDir
  - [x] Use `strings.ReplaceAll` for the `{project-root}` placeholder (not regexp)
  - [x] Clean the resulting path with `filepath.Clean` to normalize separators
- [x] Task 4: Update test fixtures in `internal/status/testdata/` (AC: #3, #4)
  - [x] Add `story_location` field to `sprint_status.yaml` fixture: `story_location: "{project-root}/_bmad-output/implementation-artifacts"`
  - [x] Leave `sprint_empty.yaml` WITHOUT `story_location` to test the missing-field error path
  - [x] Create `sprint_no_location.yaml` — valid development_status but no story_location key (explicit missing-field fixture)
- [x] Task 5: Add tests for `StoriesByStatus()` to `internal/status/reader_test.go` (AC: #1, #2)
  - [x] Test filters to correct status (backlog, done, in-progress, ready-for-dev)
  - [x] Test excludes non-story entries (epics, retrospectives)
  - [x] Test sorts by epic number then story number (numeric: 2-10 after 2-2, not after 2-1)
  - [x] Test returns empty slice when no stories match
  - [x] Test accepts non-standard status values (e.g., `optional`)
- [x] Task 6: Add tests for `ResolveStoryLocation()` to `internal/status/reader_test.go` (AC: #3, #4, #5)
  - [x] Test resolves `{project-root}` placeholder with given projectDir
  - [x] Test returns error when `story_location` field is missing
  - [x] Test with empty projectDir replaces placeholder with empty string
  - [x] Test path is cleaned (normalized with filepath.Clean)
  - [x] Test file-not-found returns error
- [x] Task 7: Update `StatusReader` interface in `internal/cli/root.go` (AC: #1, #3)
  - [x] Add `StoriesByStatus(status string) ([]status.Entry, error)` to interface
  - [x] Add `ResolveStoryLocation(projectDir string) (string, error)` to interface
  - [x] Keep existing methods: `BacklogStories()`, `StoriesForEpic(int)`, `StoryByKey(string)`
- [x] Task 8: Update `internal/status/doc_test.go`
  - [x] Add or update examples demonstrating `StoriesByStatus()` and `ResolveStoryLocation()`
- [x] Task 9: Verify `just check` passes (fmt + vet + test)

## Dev Notes

### What This Story IS

Add the remaining two query methods to the status Reader: the generic `StoriesByStatus(status)` filter and the `ResolveStoryLocation(projectDir)` path resolver. After this story, the Reader has the complete query API needed by the pipeline (Epic 2).

### What This Story is NOT

- Do NOT create the `internal/pipeline/` package (Epic 2)
- Do NOT add any CLI subcommands
- Do NOT modify `internal/claude/`, `internal/config/`, or `internal/output/`
- Do NOT add `StoriesByStatus` to the existing `BacklogStories()` or remove `BacklogStories()` — both coexist. `BacklogStories()` is a convenience wrapper; `StoriesByStatus()` is the generic version.

### Architecture Constraints

**Leaf package** — `internal/status/` depends only on stdlib + `gopkg.in/yaml.v3`. Never import other internal packages.

**Read-only contract** — Never modify sprint-status.yaml. (NFR6)

**Re-read from disk** — Every public method re-reads the file. No caching across calls. (NFR4)

**Error handling** — Infrastructure errors (file I/O, YAML parse) return `error`. Sentinel `ErrStoryNotFound` for missing entries (already exists from Story 1.2). No new sentinel errors needed for this story — `ResolveStoryLocation` returns a wrapped error for missing field, not a sentinel.

### Current Code State (from Story 1.2)

**`internal/status/types.go`:**
- `EntryType` enum: `EntryTypeEpic`, `EntryTypeStory`, `EntryTypeRetrospective` with `String()`
- `Entry` struct: `Key`, `Status`, `Type`, `EpicNum`, `StoryNum`, `Slug`
- `ErrStoryNotFound` sentinel
- `Status` type and constants (`StatusBacklog`, `StatusReadyForDev`, etc.)

**`internal/status/reader.go`:**
- `Read() ([]Entry, error)` — uses yaml.v3 Node API, preserves YAML order, classifies entries
- `BacklogStories() ([]Entry, error)` — filters stories with backlog status, sorted by epic+story
- `StoriesForEpic(n int) ([]Entry, error)` — filters stories for epic N, sorted by story number
- `StoryByKey(key string) (*Entry, error)` — lookup by key, returns `ErrStoryNotFound`
- `classifyKey(key string)` — regex matching: retrospective > epic > story

**`internal/status/testdata/`:**
- `sprint_status.yaml` — standard fixture with 3 epics, stories, retrospectives (does NOT have `story_location` — must add)
- `sprint_empty.yaml` — empty `development_status: {}` (no `story_location` — use as missing-field test)
- `sprint_unrecognized.yaml` — nested format for rejection

**`internal/cli/root.go` — `StatusReader` interface:**
```go
type StatusReader interface {
    BacklogStories() ([]status.Entry, error)
    StoriesForEpic(n int) ([]status.Entry, error)
    StoryByKey(key string) (*status.Entry, error)
}
```

### Implementation Pattern for StoriesByStatus

Follow the exact pattern from `BacklogStories()` but parameterize the status filter:

```go
func (r *Reader) StoriesByStatus(status string) ([]Entry, error) {
    entries, err := r.Read()
    if err != nil {
        return nil, err
    }
    var result []Entry
    for _, e := range entries {
        if e.Type == EntryTypeStory && string(e.Status) == status {
            result = append(result, e)
        }
    }
    sort.Slice(result, func(i, j int) bool {
        if result[i].EpicNum != result[j].EpicNum {
            return result[i].EpicNum < result[j].EpicNum
        }
        return result[i].StoryNum < result[j].StoryNum
    })
    return result, nil
}
```

Note: accepts `string` parameter (not `Status` type) so callers don't need to cast. The filter compares `string(e.Status) == status`.

### Implementation Pattern for ResolveStoryLocation

Requires reading YAML metadata that `Read()` currently skips. Add an internal helper that traverses the root mapping node to find `story_location`:

```go
func (r *Reader) readStoryLocation() (string, error) {
    fullPath := filepath.Join(r.basePath, DefaultStatusPath)
    data, err := os.ReadFile(fullPath)
    if err != nil {
        return "", fmt.Errorf("failed to read sprint status: %w", err)
    }
    var doc yaml.Node
    if err := yaml.Unmarshal(data, &doc); err != nil {
        return "", fmt.Errorf("failed to parse sprint status: %w", err)
    }
    if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
        return "", fmt.Errorf("invalid sprint status document")
    }
    root := doc.Content[0]
    if root.Kind != yaml.MappingNode {
        return "", fmt.Errorf("invalid sprint status format")
    }
    for i := 0; i < len(root.Content); i += 2 {
        if root.Content[i].Value == "story_location" {
            return root.Content[i+1].Value, nil
        }
    }
    return "", fmt.Errorf("story_location field not found in sprint status")
}

func (r *Reader) ResolveStoryLocation(projectDir string) (string, error) {
    tmpl, err := r.readStoryLocation()
    if err != nil {
        return "", err
    }
    resolved := strings.ReplaceAll(tmpl, "{project-root}", projectDir)
    return filepath.Clean(resolved), nil
}
```

### YAML story_location Format

The `sprint-status.yaml` has a top-level `story_location` field:
```yaml
story_location: "{project-root}/_bmad-output/implementation-artifacts"
```

The placeholder `{project-root}` uses curly braces (not Go template syntax). Simple string replacement is sufficient.

### Fixture Update Required

The existing `testdata/sprint_status.yaml` must gain a `story_location` field. Add it alongside the existing metadata:
```yaml
story_location: "{project-root}/_bmad-output/implementation-artifacts"
```

This does NOT affect any existing tests — the `Read()` method ignores top-level keys other than `development_status`.

### Dependencies

- No new dependencies. All changes use stdlib + `gopkg.in/yaml.v3` (already direct).
- `strings` and `path/filepath` from stdlib (already imported).

### Project Structure Notes

- All changes confined to `internal/status/` and `internal/cli/root.go`
- No new files — only modifications to existing reader.go, reader_test.go, doc_test.go, root.go, and testdata fixtures
- One new fixture file: `testdata/sprint_no_location.yaml`

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 1, Story 1.3]
- [Source: _bmad-output/planning-artifacts/architecture.md - Package: internal/status/]
- [Source: _bmad-output/planning-artifacts/architecture.md - Testing Standards]
- [Source: _bmad-output/planning-artifacts/architecture.md - Error Handling]
- [Source: _bmad-output/planning-artifacts/prd.md - FR3, FR6, NFR2, NFR4, NFR6]
- [Source: _bmad-output/implementation-artifacts/1-2-sprint-status-yaml-parser.md - Completion Notes, Dev Notes]

### Previous Story Intelligence

From Story 1.2 (sprint-status-yaml-parser):
- **Node API pattern established** — `Read()` uses `yaml.v3` Node API to traverse `development_status` mapping. `readStoryLocation()` should follow the same traversal pattern for top-level fields.
- **Sort pattern established** — `BacklogStories()` sorts by epic+story number. `StoriesByStatus()` should use identical sort logic.
- **Fixture-based testing** — Tests use `testdata/` fixtures loaded via `setupFixture()` helper. Follow the same pattern.
- **No caching** — Each public method re-reads from disk. `ResolveStoryLocation()` must also re-read (no caching the story_location value).
- **Status as string** — The `Entry.Status` field is type `Status` (a `string` typedef). Non-standard values like `optional` are stored as-is. `StoriesByStatus()` should accept any string.
- **Empty slice convention** — `Read()` returns `[]Entry{}` (not nil) for empty maps. `StoriesByStatus()` should follow the same convention when no entries match.
- **classifyKey is unexported** — Internal helper, not part of the public API. `readStoryLocation()` should also be unexported.

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

One test fix: `TestResolveStoryLocation_EmptyProjectDir` expected `_bmad-output/...` but `filepath.Clean` preserves the leading `/` from the template after empty string replacement. Fixed test expectation.

### Completion Notes List

- Added `StoriesByStatus(status string)` — generic status filter accepting any status string, returns sorted story entries
- Added `readStoryLocation()` — internal helper reads YAML file and extracts `story_location` top-level field via Node API
- Added `ResolveStoryLocation(projectDir string)` — replaces `{project-root}` placeholder with projectDir, cleans path
- Updated `sprint_status.yaml` fixture with `story_location` field
- Created `sprint_no_location.yaml` fixture for missing-field error path
- Added 8 new test functions for StoriesByStatus (backlog, done, in-progress, excludes non-stories, no match, numeric sort, empty)
- Added 6 new test functions for ResolveStoryLocation (success, empty dir, missing field, empty dev status, file not found, path cleaned)
- Updated doc_test.go with StoriesByStatus and ResolveStoryLocation examples
- Updated StatusReader interface in cli/root.go with 2 new methods
- All tests pass: `just check` (fmt + vet + test) green

### Change Log

- 2026-04-06: Implemented story 1-3 — added StoriesByStatus generic filter, ResolveStoryLocation path resolver, comprehensive tests

### File List

- internal/status/reader.go (modified)
- internal/status/reader_test.go (modified)
- internal/status/doc_test.go (modified)
- internal/status/testdata/sprint_status.yaml (modified)
- internal/status/testdata/sprint_no_location.yaml (new)
- internal/cli/root.go (modified)
