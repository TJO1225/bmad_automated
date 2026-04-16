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

// titleRegex matches "# Story <id>: Title" headings with flexible ID formats
// (e.g., "1.2", "12.5a", "12-5a") and captures the title portion after the colon.
var titleRegex = regexp.MustCompile(`(?m)^#\s+Story\s+[\w.\-]+:\s*(.+)$`)

// fallbackTitleRegex matches any H1 heading as a last resort when the
// structured "# Story X.Y: Title" pattern isn't found.
var fallbackTitleRegex = regexp.MustCompile(`(?m)^#\s+(.+)$`)

// ExtractTitle parses a story markdown file and returns the title portion.
// It first tries the structured "# Story X.Y: Title" format, then falls
// back to the raw text of the first H1 heading. Projects with non-standard
// heading formats (e.g., "# Epic 12 Phase 5a — Workflow Cleanup") are
// handled by the fallback.
func ExtractTitle(content string) (string, error) {
	if matches := titleRegex.FindStringSubmatch(content); matches != nil {
		title := strings.TrimSpace(matches[1])
		if title != "" {
			return title, nil
		}
	}
	if matches := fallbackTitleRegex.FindStringSubmatch(content); matches != nil {
		title := strings.TrimSpace(matches[1])
		if title != "" {
			return title, nil
		}
	}
	return "", fmt.Errorf("no title heading found in story file (expected '# Story X.Y: Title' or any '# Heading')")
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
