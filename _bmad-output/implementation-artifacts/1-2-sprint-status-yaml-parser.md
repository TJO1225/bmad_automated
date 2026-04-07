# Story 1.2: Sprint Status YAML Parser

Status: done

## Story

As an operator,
I want the tool to parse `sprint-status.yaml` and classify each entry,
so that the system understands which entries are epics, stories, or retrospectives.

## Acceptance Criteria

1. **Given** a valid `sprint-status.yaml` with BMAD v6 flat `development_status` format
   **When** the Reader parses the file
   **Then** all entries are loaded with their keys and status values
   **And** entries with key pattern `epic-N` are classified as epic type
   **And** entries with key pattern `N-M-slug` are classified as story type
   **And** entries with key pattern `epic-N-retrospective` are classified as retrospective type

2. **Given** a YAML file with an unrecognized format (e.g., nested `sprints[].stories[]`)
   **When** the Reader attempts to parse it
   **Then** a clear error message is returned identifying the format as unrecognized
   **And** no partial or incorrect data is produced

3. **Given** an empty `development_status` map
   **When** the Reader parses the file
   **Then** the Reader returns successfully with zero entries

4. **Given** a parsed sprint status with stories in `backlog` status
   **When** I call `BacklogStories()`
   **Then** only story entries with `backlog` status are returned
   **And** entries are sorted by epic number then story number

5. **Given** a parsed sprint status with stories across multiple epics
   **When** I call `StoriesForEpic(1)`
   **Then** only story entries belonging to epic 1 are returned
   **And** entries are sorted by story number (numeric, not lexicographic)

6. **Given** a parsed sprint status with a known story key
   **When** I call `StoryByKey("1-2-sprint-status-yaml-parser")`
   **Then** the matching entry is returned with its status and classification
   **And** calling `StoryByKey("nonexistent")` returns `ErrStoryNotFound`

## Tasks / Subtasks

- [x] Task 1: Rewrite `internal/status/types.go` (AC: #1)
  - [x] Add `EntryType` enum: `EntryTypeEpic`, `EntryTypeStory`, `EntryTypeRetrospective`
  - [x] Add `Entry` struct with `Key`, `Status`, `Type`, `EpicNum`, `StoryNum`, `Slug` fields
  - [x] Add sentinel error `ErrStoryNotFound`
  - [x] Keep existing `Status` type and constants unchanged (already correct)
  - [x] Remove package doc references to `Writer` and `internal/router`
- [x] Task 2: Rewrite `internal/status/reader.go` core parser (AC: #1, #2, #3)
  - [x] Keep `DefaultStatusPath` constant, `Reader` struct, `NewReader()` constructor
  - [x] Rewrite `Read()` to return `[]Entry` with classified types instead of `*SprintStatus`
  - [x] Preserve YAML insertion order in returned entries (use `yaml.v3` Node API to iterate the mapping, not `map[string]Status` unmarshaling which loses order)
  - [x] Add entry classification via `classifyKey(key string) (EntryType, epicNum int, storyNum int, slug string)` helper
  - [x] Add format validation: reject YAML without `development_status` map key (AC: #2)
  - [x] Handle empty `development_status` map gracefully (AC: #3)
- [x] Task 2b: Add query methods to Reader (AC: #4, #5, #6)
  - [x] Add `BacklogStories() ([]Entry, error)` — calls `Read()`, filters to story entries with `backlog` status, sorted by epic then story number
  - [x] Add `StoriesForEpic(n int) ([]Entry, error)` — calls `Read()`, filters to story entries for epic N, sorted by story number
  - [x] Add `StoryByKey(key string) (*Entry, error)` — calls `Read()`, returns single entry or `ErrStoryNotFound`
  - [x] Note: these evolve the existing `GetStoryStatus()` and `GetEpicStories()` methods with richer return types. Story 1.3 will add the generic `StoriesByStatus(status string)` and `ResolveStoryLocation()`
- [x] Task 3: Delete dead code
  - [x] Delete `internal/status/writer.go`
  - [x] Delete `internal/status/writer_test.go`
- [x] Task 4: Create test fixtures in `internal/status/testdata/` (AC: #1, #2, #3)
  - [x] `sprint_status.yaml` - standard fixture with epics, stories, retrospectives across multiple epics
  - [x] `sprint_empty.yaml` - valid file with empty `development_status: {}`
  - [x] `sprint_unrecognized.yaml` - nested/invalid format to test rejection
- [x] Task 5: Rewrite `internal/status/reader_test.go` (AC: #1, #2, #3, #4, #5, #6)
  - [x] Test `Read()` parses all entries with correct types from fixture
  - [x] Test `Read()` returns entries in YAML file order (not sorted, not random)
  - [x] Test entry classification: epic, story, retrospective patterns
  - [x] Test `BacklogStories()` filters correctly and sorts by epic+story number
  - [x] Test `StoriesForEpic()` filters and sorts by story number
  - [x] Test `StoryByKey()` found and not-found cases
  - [x] Test empty development_status returns zero entries
  - [x] Test unrecognized format returns clear error
  - [x] Test file-not-found returns error
  - [x] Test entries with non-standard status values (e.g., `optional` on retrospectives) are parsed without error
  - [x] Use table-driven tests for classification edge cases
- [x] Task 6: Update `internal/status/types_test.go`
  - [x] Add tests for `EntryType` string representation
  - [x] Keep existing `Status.IsValid()` tests
- [x] Task 7: Update `internal/status/doc_test.go`
  - [x] Replace examples to use new `Read()` and classification API
- [x] Task 8: Update `internal/cli/root.go` `StatusReader` interface (AC: #1)
  - [x] Update interface methods to match new Reader API: `BacklogStories()`, `StoriesForEpic(int)`, `StoryByKey(string)`
  - [x] Remove old `GetStoryStatus()` and `GetEpicStories()` from interface
- [x] Task 9: Verify `just check` passes (fmt + vet + test)

## Dev Notes

### What This Story IS

Rewrite the `internal/status/` package to be a **read-only, classifying parser** for BMAD v6 flat `development_status` YAML. After this story, the Reader can parse every entry, classify it by type (epic/story/retrospective), and provide query methods for downstream consumers.

### What This Story is NOT

- Do NOT create the `internal/pipeline/` package (Epic 2)
- Do NOT add path resolution or `ResolveStoryLocation()` (Story 1.3)
- Do NOT add the generic `StoriesByStatus(status string)` method (Story 1.3) — this story includes the specific `BacklogStories()` convenience method and the existing `GetStoryStatus`/`GetEpicStories` replacements, but the parameterized status filter is Story 1.3 scope
- Do NOT add any CLI subcommands
- Do NOT modify `internal/claude/`, `internal/config/`, or `internal/output/`

### Architecture Constraints

**Leaf package** - `internal/status/` must depend only on stdlib and `gopkg.in/yaml.v3`. Never import other internal packages.

**Read-only contract** - Story Factory reads `sprint-status.yaml`; BMAD writes it. The Reader must never modify the file. (NFR6)

**Re-read from disk** - Always read fresh from disk before each operation. Never cache `SprintStatus` across calls. Each public method re-reads the file. (NFR4)

**Error handling** - Infrastructure errors (file I/O, YAML parse) return `error`. Use sentinel `ErrStoryNotFound` for missing entries. Do NOT use `StepResult` (that's a pipeline concept from Epic 2).

### Metadata Fields — Not In Scope

The actual `sprint-status.yaml` has top-level metadata (`story_location`, `project`, `generated`, etc.) but the current `SprintStatus` struct ignores them — it only maps `development_status`. Removing `SprintStatus` is safe because metadata was never accessible via the parser. Story 1.3 will add `ResolveStoryLocation()` and will handle reading the `story_location` field at that time.

### Entry Ordering

`Read()` must return entries in YAML file order (insertion order), not sorted or random. This matters for batch processing (Epic 3: sequential story processing in key order). Use `yaml.v3`'s Node API to iterate the `development_status` mapping node's content pairs, rather than unmarshaling into `map[string]Status` (which loses order in Go maps).

### Non-Standard Status Values

Retrospective entries use `optional` status (e.g., `epic-1-retrospective: optional`), which is not in the `Status` constants. The `IsValid()` method returns false for `optional`. This is acceptable — the Reader should store the raw status string and NOT reject entries with unknown status values. Classification and parsing should work regardless of the status value. Validation of status values is a consumer concern, not the parser's.

### Entry Classification Logic

Match key patterns in this exact order (retrospective before epic to avoid false match):

| Pattern | Type | Regex | Examples |
|---------|------|-------|----------|
| `epic-N-retrospective` | Retrospective | `^epic-(\d+)-retrospective$` | `epic-1-retrospective` |
| `epic-N` | Epic | `^epic-(\d+)$` | `epic-1`, `epic-2` |
| `N-M-slug` | Story | `^(\d+)-(\d+)-(.+)$` | `1-2-sprint-status-yaml-parser` |

Keys that don't match any pattern should be treated as unclassified and included in results with a sensible default or logged as a warning - do NOT silently drop them.

### BMAD v6 YAML Format

The flat `development_status` map looks like this:

```yaml
development_status:
  epic-1: in-progress
  1-1-strip-legacy-code: done
  1-2-sprint-status-yaml-parser: backlog
  epic-1-retrospective: optional
  epic-2: backlog
  2-1-precondition-verification: backlog
```

Keys are strings, values are status strings. There is NO nesting. Reject any file with nested structures under `development_status`.

### Existing Code to Preserve

- `Status` type and its constants (`StatusBacklog`, `StatusReadyForDev`, etc.) in `types.go` - **keep as-is**
- `Status.IsValid()` method - **keep as-is**
- `DefaultStatusPath` constant in `reader.go` - **keep as-is**
- `Reader` struct and `NewReader()` constructor - **keep signature, may adjust internals**

### Existing Code to Remove

- `writer.go` and `writer_test.go` - dead code, Story 1.1 left these for us to clean up
- `SprintStatus` struct in `types.go` - replaced by `[]Entry` return from `Read()`
- `GetStoryStatus()` method - replaced by `StoryByKey()`
- `GetEpicStories()` method - replaced by `StoriesForEpic()`
- Package doc references to `Writer` and `internal/router`
- Doc references to `internal/router` package (deleted in Story 1.1, deferred to us)

### New Types to Create

```go
// EntryType classifies a development_status entry.
type EntryType int

const (
    EntryTypeEpic          EntryType = iota
    EntryTypeStory
    EntryTypeRetrospective
)

// Entry represents a single parsed entry from the development_status map.
type Entry struct {
    Key      string    // Raw key, e.g. "1-2-sprint-status-yaml-parser"
    Status   Status    // Parsed status value
    Type     EntryType // Classified type
    EpicNum  int       // Epic number (all types have this)
    StoryNum int       // Story number (only EntryTypeStory, 0 for others)
    Slug     string    // Slug portion (only EntryTypeStory, empty for others)
}
```

### New Reader Methods

```go
// Read parses sprint-status.yaml and returns all classified entries.
// Returns error if file not found, YAML invalid, or format unrecognized.
func (r *Reader) Read() ([]Entry, error)

// BacklogStories returns all story entries with backlog status, sorted by epic then story number.
func (r *Reader) BacklogStories() ([]Entry, error)

// StoriesForEpic returns all story entries for the given epic number, sorted by story number.
func (r *Reader) StoriesForEpic(n int) ([]Entry, error)

// StoryByKey returns the entry matching the given key, or ErrStoryNotFound.
func (r *Reader) StoryByKey(key string) (*Entry, error)
```

### CLI Interface Update

The `StatusReader` interface in `internal/cli/root.go` must be updated to match. The old methods `GetStoryStatus()` and `GetEpicStories()` are replaced:

```go
// Before (current):
type StatusReader interface {
    GetStoryStatus(storyKey string) (status.Status, error)
    GetEpicStories(epicID string) ([]string, error)
}

// After (this story):
type StatusReader interface {
    BacklogStories() ([]status.Entry, error)
    StoriesForEpic(n int) ([]status.Entry, error)
    StoryByKey(key string) (*status.Entry, error)
}
```

Note: This interface is interim — Story 1.3 will add `StoriesByStatus(status string)` and `ResolveStoryLocation(projectDir string)` methods.

### Testing Strategy

- Use `testdata/` fixture files (not inline YAML strings in tests)
- Table-driven tests for classification: test all 3 patterns plus edge cases
- Edge cases to cover:
  - Story number `10` sorts after `9` not after `1` (numeric sort, not lexicographic)
  - Keys with extra dashes in slug: `1-2-my-long-slug-name`
  - Empty development_status map
  - File with only epics (no stories)
  - File with only retrospectives
  - Missing `development_status` key entirely
  - `development_status` key present but value is not a map
- Use `t.TempDir()` for file-based tests OR fixture files in `testdata/` - fixtures preferred for readability
- `testify/assert` for soft checks, `testify/require` for fatal preconditions

### Dependencies

- `gopkg.in/yaml.v3` v3.0.1 (already a direct dependency in go.mod)
- `github.com/stretchr/testify` v1.11.1 (already in go.mod, for tests)
- No new dependencies needed

### Project Structure Notes

- All changes confined to `internal/status/` and `internal/cli/root.go`
- Alignment with architecture: status is a leaf package, no internal imports
- New `testdata/` directory matches pattern from `internal/claude/testdata/`

### References

- [Source: _bmad-output/planning-artifacts/epics.md - Epic 1, Story 1.2]
- [Source: _bmad-output/planning-artifacts/architecture.md - Package: internal/status/]
- [Source: _bmad-output/planning-artifacts/architecture.md - Testing Standards]
- [Source: _bmad-output/planning-artifacts/architecture.md - Error Handling]
- [Source: _bmad-output/planning-artifacts/prd.md - FR1-FR6, NFR2, NFR4, NFR6]
- [Source: _bmad-output/implementation-artifacts/1-1-strip-legacy-code-and-rename-project.md - Completion Notes]

### Previous Story Intelligence

From Story 1.1 (strip-legacy-code):
- **Writer left as dead code** - Story 1.1 explicitly deferred writer deletion to us. Delete `writer.go` and `writer_test.go`.
- **types.go godoc references `internal/router`** - The review deferred fixing this to Story 1.2. Remove the stale reference.
- **Import paths already renamed** to `story-factory` - no module rename work needed.
- **`go mod tidy` promoted yaml.v3 to direct** - already available, no dependency changes needed.
- **DefaultConfig() still has deleted workflows** - not our concern (deferred to later story).
- **`CycleHeader()` hardcodes deleted pipeline** - not our concern (output package, deferred).

## Dev Agent Record

### Agent Model Used

Claude Opus 4.6 (1M context)

### Debug Log References

None — clean implementation with no blocking issues.

### Completion Notes List

- Rewrote `internal/status/types.go`: added `EntryType` enum with `String()`, `Entry` struct, `ErrStoryNotFound` sentinel; removed `SprintStatus` struct and stale doc references to `Writer`/`internal/router`
- Rewrote `internal/status/reader.go`: `Read()` now returns `[]Entry` using yaml.v3 Node API to preserve YAML insertion order; added `classifyKey()` with regex matching (retrospective > epic > story priority); format validation rejects missing/non-mapping `development_status`
- Added query methods: `BacklogStories()` (filters+sorts by epic+story), `StoriesForEpic(n)` (filters+sorts by story num), `StoryByKey()` (returns `ErrStoryNotFound` sentinel)
- Deleted dead code: `writer.go`, `writer_test.go`
- Created 3 test fixtures in `testdata/`: standard, empty, unrecognized format
- Rewrote `reader_test.go` with 22 test functions covering all ACs, edge cases, table-driven classification tests
- Added `EntryType.String()` tests to `types_test.go`
- Updated `doc_test.go` examples to use new `Read()` and classification API
- Updated `StatusReader` interface in `cli/root.go` to match new API
- All tests pass: `just check` (fmt + vet + test) green

### Change Log

- 2026-04-06: Implemented story 1-2 — rewrote status package as read-only classifying parser with entry classification, query methods, and comprehensive test coverage

### File List

- internal/status/types.go (modified)
- internal/status/reader.go (modified)
- internal/status/reader_test.go (modified)
- internal/status/types_test.go (modified)
- internal/status/doc_test.go (modified)
- internal/status/writer.go (deleted)
- internal/status/writer_test.go (deleted)
- internal/status/testdata/sprint_status.yaml (new)
- internal/status/testdata/sprint_empty.yaml (new)
- internal/status/testdata/sprint_unrecognized.yaml (new)
- internal/cli/root.go (modified)
