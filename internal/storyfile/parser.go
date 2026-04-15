// Package storyfile parses BMAD story markdown files.
//
// Story files produced by /bmad-create-story follow a consistent shape:
//
//	# Story X.Y: <Title>
//
//	Status: <status>
//
//	## Story
//	... user story prose ...
//
//	## Acceptance Criteria
//	... numbered ACs in Given/When/Then form ...
//
//	## Tasks / Subtasks
//	... task checklist ...
//
// This package exposes helpers that both beads-sync and git-commit steps
// use to pull human-readable details out of the file without duplicating
// regex logic.
package storyfile

import (
	"fmt"
	"regexp"
	"strings"
)

// titleRegex matches "# Story X.Y: Title" headings and captures the title portion.
var titleRegex = regexp.MustCompile(`(?m)^#\s+Story\s+\d+\.\d+:\s*(.+)$`)

// ExtractTitle parses a story markdown file and returns the title portion
// from the "# Story X.Y: Title" heading. For example, given
// "# Story 1.2: Database Schema", it returns "Database Schema".
func ExtractTitle(content string) (string, error) {
	matches := titleRegex.FindStringSubmatch(content)
	if matches == nil {
		return "", fmt.Errorf("no story title heading found (expected '# Story X.Y: Title')")
	}
	title := strings.TrimSpace(matches[1])
	if title == "" {
		return "", fmt.Errorf("story title heading has empty title")
	}
	return title, nil
}

// ExtractAcceptanceCriteria extracts the content of the "## Acceptance Criteria"
// section from a story markdown file. It captures all content between the
// "## Acceptance Criteria" heading (with optional suffix like "(BDD)") and
// the next "## " heading (or EOF).
func ExtractAcceptanceCriteria(content string) (string, error) {
	acRegex := regexp.MustCompile(`(?m)^## Acceptance Criteria[^\n]*$`)
	loc := acRegex.FindStringIndex(content)
	if loc == nil {
		return "", fmt.Errorf("no '## Acceptance Criteria' section found")
	}

	headingEnd := loc[1]

	start := headingEnd
	if nlIdx := strings.Index(content[start:], "\n"); nlIdx != -1 {
		start += nlIdx + 1
	} else {
		return "", fmt.Errorf("acceptance criteria section is empty")
	}

	rest := content[start:]
	endIdx := strings.Index(rest, "\n## ")
	var body string
	if endIdx == -1 {
		body = rest
	} else {
		body = rest[:endIdx]
	}

	trimmed := strings.TrimSpace(body)
	if trimmed == "" {
		return "", fmt.Errorf("acceptance criteria section is empty")
	}

	return trimmed, nil
}
