package status

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultStatusPath is the canonical location of the sprint-status.yaml file
// relative to the project root. This path is used by [Reader].
const DefaultStatusPath = "_bmad-output/implementation-artifacts/sprint-status.yaml"

// Classification regexes, checked in order: retrospective before epic to avoid false match.
var (
	retroRe = regexp.MustCompile(`^epic-(\d+)-retrospective$`)
	epicRe  = regexp.MustCompile(`^epic-(\d+)$`)
	storyRe = regexp.MustCompile(`^(\d+)-(\d+)-(.+)$`)
)

// Reader reads sprint status from YAML files at [DefaultStatusPath].
//
// The basePath field specifies the project root directory. When empty,
// the current working directory is used. The full path to the status file
// is constructed as: basePath + DefaultStatusPath.
type Reader struct {
	basePath string
}

// NewReader creates a new [Reader] with the specified base path.
//
// The basePath is the project root directory. Pass an empty string to use
// the current working directory.
func NewReader(basePath string) *Reader {
	return &Reader{
		basePath: basePath,
	}
}

// Read parses sprint-status.yaml and returns all classified entries in YAML file order.
//
// It uses yaml.v3's Node API to preserve insertion order. Each entry is classified
// by key pattern into epic, story, or retrospective type. Keys that don't match
// any known pattern are included with EntryTypeStory as default and zero numeric fields.
//
// Returns an error if the file cannot be read, YAML is invalid, or the format
// is unrecognized (missing or non-mapping development_status).
func (r *Reader) Read() ([]Entry, error) {
	fullPath := filepath.Join(r.basePath, DefaultStatusPath)

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read sprint status: %w", err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("failed to parse sprint status: %w", err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return nil, fmt.Errorf("unrecognized sprint status format: empty document")
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("unrecognized sprint status format: expected mapping at root")
	}

	// Find development_status key in root mapping
	var devStatusNode *yaml.Node
	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "development_status" {
			devStatusNode = root.Content[i+1]
			break
		}
	}

	if devStatusNode == nil {
		return nil, fmt.Errorf("unrecognized sprint status format: missing development_status key")
	}

	if devStatusNode.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("unrecognized sprint status format: development_status is not a mapping")
	}

	// Empty map — valid, zero entries
	if len(devStatusNode.Content) == 0 {
		return []Entry{}, nil
	}

	entries := make([]Entry, 0, len(devStatusNode.Content)/2)
	for i := 0; i < len(devStatusNode.Content); i += 2 {
		key := devStatusNode.Content[i].Value
		val := devStatusNode.Content[i+1].Value

		entryType, epicNum, storyNum, slug := classifyKey(key)
		entries = append(entries, Entry{
			Key:      key,
			Status:   Status(val),
			Type:     entryType,
			EpicNum:  epicNum,
			StoryNum: storyNum,
			Slug:     slug,
		})
	}

	return entries, nil
}

// BacklogStories returns all story entries with backlog status, sorted by epic then story number.
func (r *Reader) BacklogStories() ([]Entry, error) {
	entries, err := r.Read()
	if err != nil {
		return nil, err
	}

	result := []Entry{}
	for _, e := range entries {
		if e.Type == EntryTypeStory && e.Status == StatusBacklog {
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

// StoriesForEpic returns all story entries for the given epic number, sorted by story number.
func (r *Reader) StoriesForEpic(n int) ([]Entry, error) {
	entries, err := r.Read()
	if err != nil {
		return nil, err
	}

	result := []Entry{}
	for _, e := range entries {
		if e.Type == EntryTypeStory && e.EpicNum == n {
			result = append(result, e)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].StoryNum < result[j].StoryNum
	})

	return result, nil
}

// StoriesByStatus returns all story entries matching the given status string,
// sorted by epic number then story number.
func (r *Reader) StoriesByStatus(status string) ([]Entry, error) {
	entries, err := r.Read()
	if err != nil {
		return nil, err
	}

	result := []Entry{}
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

// StoryByKey returns the entry matching the given key, or [ErrStoryNotFound].
func (r *Reader) StoryByKey(key string) (*Entry, error) {
	entries, err := r.Read()
	if err != nil {
		return nil, err
	}

	for i := range entries {
		if entries[i].Key == key {
			return &entries[i], nil
		}
	}

	return nil, ErrStoryNotFound
}

// ResolveStoryLocation reads the story_location field from sprint-status.yaml
// and resolves the {project-root} placeholder with the given project directory.
func (r *Reader) ResolveStoryLocation(projectDir string) (string, error) {
	tmpl, err := r.readStoryLocation()
	if err != nil {
		return "", err
	}

	resolved := strings.ReplaceAll(tmpl, "{project-root}", projectDir)
	return filepath.Clean(resolved), nil
}

// readStoryLocation reads the YAML file and extracts the story_location top-level field.
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
		return "", fmt.Errorf("story_location field not found in sprint status")
	}

	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return "", fmt.Errorf("story_location field not found in sprint status")
	}

	for i := 0; i < len(root.Content); i += 2 {
		if root.Content[i].Value == "story_location" {
			node := root.Content[i+1]
			if node.Kind != yaml.ScalarNode || strings.TrimSpace(node.Value) == "" {
				return "", fmt.Errorf("story_location field is empty in sprint status")
			}
			return node.Value, nil
		}
	}

	return "", fmt.Errorf("story_location field not found in sprint status")
}

// classifyKey determines the entry type and extracts numeric fields from the key.
// Order matters: retrospective is checked before epic to avoid false match.
func classifyKey(key string) (EntryType, int, int, string) {
	if m := retroRe.FindStringSubmatch(key); m != nil {
		epicNum, _ := strconv.Atoi(m[1])
		return EntryTypeRetrospective, epicNum, 0, ""
	}
	if m := epicRe.FindStringSubmatch(key); m != nil {
		epicNum, _ := strconv.Atoi(m[1])
		return EntryTypeEpic, epicNum, 0, ""
	}
	if m := storyRe.FindStringSubmatch(key); m != nil {
		epicNum, _ := strconv.Atoi(m[1])
		storyNum, _ := strconv.Atoi(m[2])
		return EntryTypeStory, epicNum, storyNum, m[3]
	}
	// Unclassified key — include with default type, zero fields
	return EntryTypeStory, 0, 0, ""
}
