package beads

import "story-factory/internal/storyfile"

// ExtractTitle delegates to [storyfile.ExtractTitle] for backwards
// compatibility. New callers should import storyfile directly.
func ExtractTitle(content string) (string, error) {
	return storyfile.ExtractTitle(content)
}

// ExtractAcceptanceCriteria delegates to [storyfile.ExtractAcceptanceCriteria]
// for backwards compatibility. New callers should import storyfile directly.
func ExtractAcceptanceCriteria(content string) (string, error) {
	return storyfile.ExtractAcceptanceCriteria(content)
}
