package status

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper copies a testdata fixture into a temp directory at the DefaultStatusPath location.
func setupFixture(t *testing.T, fixtureName string) string {
	t.Helper()
	tmpDir := t.TempDir()
	statusDir := filepath.Join(tmpDir, "_bmad-output", "implementation-artifacts")
	require.NoError(t, os.MkdirAll(statusDir, 0755))

	src, err := os.ReadFile(filepath.Join("testdata", fixtureName))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(statusDir, "sprint-status.yaml"), src, 0644))

	return tmpDir
}

func TestNewReader(t *testing.T) {
	reader := NewReader("/some/path")

	assert.NotNil(t, reader)
	assert.Equal(t, "/some/path", reader.basePath)
}

// --- Read() tests ---

func TestRead_ParsesAllEntries(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	entries, err := reader.Read()
	require.NoError(t, err)
	assert.Len(t, entries, 13)
}

func TestRead_CorrectTypes(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	entries, err := reader.Read()
	require.NoError(t, err)

	typeMap := make(map[string]EntryType)
	for _, e := range entries {
		typeMap[e.Key] = e.Type
	}

	// Epics
	assert.Equal(t, EntryTypeEpic, typeMap["epic-1"])
	assert.Equal(t, EntryTypeEpic, typeMap["epic-2"])
	assert.Equal(t, EntryTypeEpic, typeMap["epic-3"])

	// Retrospectives
	assert.Equal(t, EntryTypeRetrospective, typeMap["epic-1-retrospective"])
	assert.Equal(t, EntryTypeRetrospective, typeMap["epic-2-retrospective"])
	assert.Equal(t, EntryTypeRetrospective, typeMap["epic-3-retrospective"])

	// Stories
	assert.Equal(t, EntryTypeStory, typeMap["1-1-define-schema"])
	assert.Equal(t, EntryTypeStory, typeMap["1-2-create-api"])
	assert.Equal(t, EntryTypeStory, typeMap["2-1-setup-auth"])
	assert.Equal(t, EntryTypeStory, typeMap["2-10-final-cleanup"])
	assert.Equal(t, EntryTypeStory, typeMap["3-1-monitoring"])
}

func TestRead_PreservesYAMLOrder(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	entries, err := reader.Read()
	require.NoError(t, err)

	// Verify entries are in YAML file order
	keys := make([]string, len(entries))
	for i, e := range entries {
		keys[i] = e.Key
	}

	expected := []string{
		"epic-1",
		"1-1-define-schema",
		"1-2-create-api",
		"1-3-build-ui",
		"epic-1-retrospective",
		"epic-2",
		"2-1-setup-auth",
		"2-2-add-roles",
		"2-10-final-cleanup",
		"epic-2-retrospective",
		"epic-3",
		"3-1-monitoring",
		"epic-3-retrospective",
	}
	assert.Equal(t, expected, keys)
}

func TestRead_EntryFields(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	entries, err := reader.Read()
	require.NoError(t, err)

	// Check a story entry has all fields populated
	var story *Entry
	for i, e := range entries {
		if e.Key == "2-10-final-cleanup" {
			story = &entries[i]
			break
		}
	}
	require.NotNil(t, story)
	assert.Equal(t, EntryTypeStory, story.Type)
	assert.Equal(t, Status("backlog"), story.Status)
	assert.Equal(t, 2, story.EpicNum)
	assert.Equal(t, 10, story.StoryNum)
	assert.Equal(t, "final-cleanup", story.Slug)

	// Check an epic entry
	var epic *Entry
	for i, e := range entries {
		if e.Key == "epic-1" {
			epic = &entries[i]
			break
		}
	}
	require.NotNil(t, epic)
	assert.Equal(t, EntryTypeEpic, epic.Type)
	assert.Equal(t, Status("in-progress"), epic.Status)
	assert.Equal(t, 1, epic.EpicNum)
	assert.Equal(t, 0, epic.StoryNum)
	assert.Equal(t, "", epic.Slug)

	// Check a retrospective entry
	var retro *Entry
	for i, e := range entries {
		if e.Key == "epic-2-retrospective" {
			retro = &entries[i]
			break
		}
	}
	require.NotNil(t, retro)
	assert.Equal(t, EntryTypeRetrospective, retro.Type)
	assert.Equal(t, Status("optional"), retro.Status)
	assert.Equal(t, 2, retro.EpicNum)
	assert.Equal(t, 0, retro.StoryNum)
	assert.Equal(t, "", retro.Slug)
}

func TestRead_EmptyDevelopmentStatus(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_empty.yaml")
	reader := NewReader(tmpDir)

	entries, err := reader.Read()
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.NotNil(t, entries) // should be empty slice, not nil
}

func TestRead_UnrecognizedFormat(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_unrecognized.yaml")
	reader := NewReader(tmpDir)

	entries, err := reader.Read()
	assert.Error(t, err)
	assert.Nil(t, entries)
	assert.Contains(t, err.Error(), "unrecognized sprint status format")
}

func TestRead_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewReader(tmpDir)

	entries, err := reader.Read()
	assert.Error(t, err)
	assert.Nil(t, entries)
	assert.Contains(t, err.Error(), "failed to read sprint status")
}

func TestRead_NonStandardStatusValues(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	entries, err := reader.Read()
	require.NoError(t, err)

	// Retrospectives have "optional" status which is not in Status constants
	for _, e := range entries {
		if e.Key == "epic-1-retrospective" {
			assert.Equal(t, Status("optional"), e.Status)
			assert.False(t, e.Status.IsValid())
			break
		}
	}
}

func TestRead_DevelopmentStatusNotMap(t *testing.T) {
	tmpDir := t.TempDir()
	statusDir := filepath.Join(tmpDir, "_bmad-output", "implementation-artifacts")
	require.NoError(t, os.MkdirAll(statusDir, 0755))

	content := "development_status: not-a-map\n"
	require.NoError(t, os.WriteFile(filepath.Join(statusDir, "sprint-status.yaml"), []byte(content), 0644))

	reader := NewReader(tmpDir)
	entries, err := reader.Read()
	assert.Error(t, err)
	assert.Nil(t, entries)
	assert.Contains(t, err.Error(), "unrecognized sprint status format")
}

// --- classifyKey() table-driven tests ---

func TestClassifyKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		wantType EntryType
		wantEpic int
		wantNum  int
		wantSlug string
	}{
		{
			name:     "epic",
			key:      "epic-1",
			wantType: EntryTypeEpic,
			wantEpic: 1,
		},
		{
			name:     "epic large number",
			key:      "epic-42",
			wantType: EntryTypeEpic,
			wantEpic: 42,
		},
		{
			name:     "retrospective",
			key:      "epic-1-retrospective",
			wantType: EntryTypeRetrospective,
			wantEpic: 1,
		},
		{
			name:     "retrospective large number",
			key:      "epic-10-retrospective",
			wantType: EntryTypeRetrospective,
			wantEpic: 10,
		},
		{
			name:     "story simple",
			key:      "1-1-foo",
			wantType: EntryTypeStory,
			wantEpic: 1,
			wantNum:  1,
			wantSlug: "foo",
		},
		{
			name:     "story with long slug",
			key:      "1-2-my-long-slug-name",
			wantType: EntryTypeStory,
			wantEpic: 1,
			wantNum:  2,
			wantSlug: "my-long-slug-name",
		},
		{
			name:     "story double-digit numbers",
			key:      "12-10-cleanup",
			wantType: EntryTypeStory,
			wantEpic: 12,
			wantNum:  10,
			wantSlug: "cleanup",
		},
		{
			name:     "unclassified key defaults to story with zero fields",
			key:      "something-unexpected",
			wantType: EntryTypeStory,
			wantEpic: 0,
			wantNum:  0,
			wantSlug: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotType, gotEpic, gotNum, gotSlug := classifyKey(tt.key)
			assert.Equal(t, tt.wantType, gotType)
			assert.Equal(t, tt.wantEpic, gotEpic)
			assert.Equal(t, tt.wantNum, gotNum)
			assert.Equal(t, tt.wantSlug, gotSlug)
		})
	}
}

// --- BacklogStories() tests ---

func TestBacklogStories_FiltersAndSorts(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	stories, err := reader.BacklogStories()
	require.NoError(t, err)

	// Should only include stories with backlog status, sorted by epic then story number
	keys := make([]string, len(stories))
	for i, s := range stories {
		keys[i] = s.Key
	}

	expected := []string{
		"1-3-build-ui",
		"2-1-setup-auth",
		"2-2-add-roles",
		"2-10-final-cleanup",
	}
	assert.Equal(t, expected, keys)

	// Verify all are story type with backlog status
	for _, s := range stories {
		assert.Equal(t, EntryTypeStory, s.Type)
		assert.Equal(t, StatusBacklog, s.Status)
	}
}

func TestBacklogStories_Empty(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_empty.yaml")
	reader := NewReader(tmpDir)

	stories, err := reader.BacklogStories()
	require.NoError(t, err)
	assert.Empty(t, stories)
	assert.NotNil(t, stories) // must be []Entry{}, not nil
}

// --- StoriesForEpic() tests ---

func TestStoriesForEpic_FiltersAndSorts(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	stories, err := reader.StoriesForEpic(2)
	require.NoError(t, err)

	keys := make([]string, len(stories))
	for i, s := range stories {
		keys[i] = s.Key
	}

	// Story 2-10 should sort after 2-2 (numeric, not lexicographic)
	expected := []string{
		"2-1-setup-auth",
		"2-2-add-roles",
		"2-10-final-cleanup",
	}
	assert.Equal(t, expected, keys)

	for _, s := range stories {
		assert.Equal(t, EntryTypeStory, s.Type)
		assert.Equal(t, 2, s.EpicNum)
	}
}

func TestStoriesForEpic_NoStories(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	stories, err := reader.StoriesForEpic(99)
	require.NoError(t, err)
	assert.Empty(t, stories)
	assert.NotNil(t, stories) // must be []Entry{}, not nil
}

func TestStoriesForEpic_SingleStory(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	stories, err := reader.StoriesForEpic(3)
	require.NoError(t, err)
	require.Len(t, stories, 1)
	assert.Equal(t, "3-1-monitoring", stories[0].Key)
}

// --- StoryByKey() tests ---

func TestStoryByKey_Found(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	entry, err := reader.StoryByKey("1-2-create-api")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, "1-2-create-api", entry.Key)
	assert.Equal(t, StatusReadyForDev, entry.Status)
	assert.Equal(t, EntryTypeStory, entry.Type)
	assert.Equal(t, 1, entry.EpicNum)
	assert.Equal(t, 2, entry.StoryNum)
	assert.Equal(t, "create-api", entry.Slug)
}

func TestStoryByKey_NotFound(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	entry, err := reader.StoryByKey("nonexistent")
	assert.ErrorIs(t, err, ErrStoryNotFound)
	assert.Nil(t, entry)
}

func TestStoryByKey_CanFindEpicByKey(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	entry, err := reader.StoryByKey("epic-1")
	require.NoError(t, err)
	require.NotNil(t, entry)
	assert.Equal(t, EntryTypeEpic, entry.Type)
	assert.Equal(t, Status("in-progress"), entry.Status)
}

func TestStoryByKey_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewReader(tmpDir)

	entry, err := reader.StoryByKey("any-key")
	assert.Error(t, err)
	assert.Nil(t, entry)
	assert.Contains(t, err.Error(), "failed to read sprint status")
}

// --- StoriesByStatus() tests ---

func TestStoriesByStatus_Backlog(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	stories, err := reader.StoriesByStatus("backlog")
	require.NoError(t, err)

	keys := make([]string, len(stories))
	for i, s := range stories {
		keys[i] = s.Key
	}

	expected := []string{
		"1-3-build-ui",
		"2-1-setup-auth",
		"2-2-add-roles",
		"2-10-final-cleanup",
	}
	assert.Equal(t, expected, keys)
}

func TestStoriesByStatus_Done(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	stories, err := reader.StoriesByStatus("done")
	require.NoError(t, err)
	require.Len(t, stories, 1)
	assert.Equal(t, "1-1-define-schema", stories[0].Key)
}

func TestStoriesByStatus_InProgress(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	stories, err := reader.StoriesByStatus("in-progress")
	require.NoError(t, err)
	require.Len(t, stories, 1)
	assert.Equal(t, "3-1-monitoring", stories[0].Key)
}

func TestStoriesByStatus_ExcludesNonStoryEntries(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	// "backlog" appears on epics too (epic-2, epic-3), but only stories returned
	stories, err := reader.StoriesByStatus("backlog")
	require.NoError(t, err)
	for _, s := range stories {
		assert.Equal(t, EntryTypeStory, s.Type)
	}
}

func TestStoriesByStatus_NoMatch(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	stories, err := reader.StoriesByStatus("review")
	require.NoError(t, err)
	assert.Empty(t, stories)
	assert.NotNil(t, stories) // must be []Entry{}, not nil
}

func TestStoriesByStatus_NumericSort(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	stories, err := reader.StoriesByStatus("backlog")
	require.NoError(t, err)

	// 2-10 should come after 2-2, not after 2-1
	var epicTwoKeys []string
	for _, s := range stories {
		if s.EpicNum == 2 {
			epicTwoKeys = append(epicTwoKeys, s.Key)
		}
	}
	assert.Equal(t, []string{"2-1-setup-auth", "2-2-add-roles", "2-10-final-cleanup"}, epicTwoKeys)
}

func TestStoriesByStatus_Empty(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_empty.yaml")
	reader := NewReader(tmpDir)

	stories, err := reader.StoriesByStatus("backlog")
	require.NoError(t, err)
	assert.Empty(t, stories)
	assert.NotNil(t, stories) // must be []Entry{}, not nil
}

// --- ResolveStoryLocation() tests ---

func TestResolveStoryLocation_Success(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	resolved, err := reader.ResolveStoryLocation("/home/tom/projects/my-app")
	require.NoError(t, err)
	assert.Equal(t, "/home/tom/projects/my-app/_bmad-output/implementation-artifacts", resolved)
}

func TestResolveStoryLocation_EmptyProjectDir(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_status.yaml")
	reader := NewReader(tmpDir)

	resolved, err := reader.ResolveStoryLocation("")
	require.NoError(t, err)
	// Empty projectDir replaces {project-root} with "", leaving "/_bmad-output/..."
	// filepath.Clean preserves the leading slash
	assert.Equal(t, "/_bmad-output/implementation-artifacts", resolved)
}

func TestResolveStoryLocation_MissingField(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_no_location.yaml")
	reader := NewReader(tmpDir)

	resolved, err := reader.ResolveStoryLocation("/some/path")
	assert.Error(t, err)
	assert.Empty(t, resolved)
	assert.Contains(t, err.Error(), "story_location field not found")
}

func TestResolveStoryLocation_MissingFieldEmptyDevStatus(t *testing.T) {
	tmpDir := setupFixture(t, "sprint_empty.yaml")
	reader := NewReader(tmpDir)

	resolved, err := reader.ResolveStoryLocation("/some/path")
	assert.Error(t, err)
	assert.Empty(t, resolved)
	assert.Contains(t, err.Error(), "story_location field not found")
}

func TestResolveStoryLocation_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	reader := NewReader(tmpDir)

	resolved, err := reader.ResolveStoryLocation("/some/path")
	assert.Error(t, err)
	assert.Empty(t, resolved)
	assert.Contains(t, err.Error(), "failed to read sprint status")
}

func TestResolveStoryLocation_PathCleaned(t *testing.T) {
	tmpDir := t.TempDir()
	statusDir := filepath.Join(tmpDir, "_bmad-output", "implementation-artifacts")
	require.NoError(t, os.MkdirAll(statusDir, 0755))

	// story_location with trailing slash to test filepath.Clean
	content := "story_location: \"{project-root}/_bmad-output/implementation-artifacts/\"\ndevelopment_status: {}\n"
	require.NoError(t, os.WriteFile(filepath.Join(statusDir, "sprint-status.yaml"), []byte(content), 0644))

	reader := NewReader(tmpDir)
	resolved, err := reader.ResolveStoryLocation("/home/tom")
	require.NoError(t, err)
	// filepath.Clean removes trailing slash
	assert.Equal(t, "/home/tom/_bmad-output/implementation-artifacts", resolved)
}

func TestResolveStoryLocation_EmptyValue(t *testing.T) {
	tmpDir := t.TempDir()
	statusDir := filepath.Join(tmpDir, "_bmad-output", "implementation-artifacts")
	require.NoError(t, os.MkdirAll(statusDir, 0755))

	content := "story_location: \"\"\ndevelopment_status: {}\n"
	require.NoError(t, os.WriteFile(filepath.Join(statusDir, "sprint-status.yaml"), []byte(content), 0644))

	reader := NewReader(tmpDir)
	resolved, err := reader.ResolveStoryLocation("/some/path")
	assert.Error(t, err)
	assert.Empty(t, resolved)
	assert.Contains(t, err.Error(), "story_location field is empty")
}

// --- Edge case: file with only epics ---

func TestRead_OnlyEpics(t *testing.T) {
	tmpDir := t.TempDir()
	statusDir := filepath.Join(tmpDir, "_bmad-output", "implementation-artifacts")
	require.NoError(t, os.MkdirAll(statusDir, 0755))

	content := "development_status:\n  epic-1: backlog\n  epic-2: in-progress\n"
	require.NoError(t, os.WriteFile(filepath.Join(statusDir, "sprint-status.yaml"), []byte(content), 0644))

	reader := NewReader(tmpDir)
	entries, err := reader.Read()
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	for _, e := range entries {
		assert.Equal(t, EntryTypeEpic, e.Type)
	}
}

// --- Edge case: file with only retrospectives ---

func TestRead_OnlyRetrospectives(t *testing.T) {
	tmpDir := t.TempDir()
	statusDir := filepath.Join(tmpDir, "_bmad-output", "implementation-artifacts")
	require.NoError(t, os.MkdirAll(statusDir, 0755))

	content := "development_status:\n  epic-1-retrospective: optional\n  epic-2-retrospective: done\n"
	require.NoError(t, os.WriteFile(filepath.Join(statusDir, "sprint-status.yaml"), []byte(content), 0644))

	reader := NewReader(tmpDir)
	entries, err := reader.Read()
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	for _, e := range entries {
		assert.Equal(t, EntryTypeRetrospective, e.Type)
	}
}

// --- Edge case: missing development_status key entirely ---

func TestRead_MissingDevelopmentStatusKey(t *testing.T) {
	tmpDir := t.TempDir()
	statusDir := filepath.Join(tmpDir, "_bmad-output", "implementation-artifacts")
	require.NoError(t, os.MkdirAll(statusDir, 0755))

	content := "project: test\ngenerated: 2026-01-01\n"
	require.NoError(t, os.WriteFile(filepath.Join(statusDir, "sprint-status.yaml"), []byte(content), 0644))

	reader := NewReader(tmpDir)
	entries, err := reader.Read()
	assert.Error(t, err)
	assert.Nil(t, entries)
	assert.Contains(t, err.Error(), "unrecognized sprint status format")
}
