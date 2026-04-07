package beads

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
// "## Acceptance Criteria" heading and the next "## " heading (or EOF).
func ExtractAcceptanceCriteria(content string) (string, error) {
	const heading = "## Acceptance Criteria"

	idx := strings.Index(content, heading)
	if idx == -1 {
		return "", fmt.Errorf("no '## Acceptance Criteria' section found")
	}

	// Start after the heading line
	start := idx + len(heading)
	// Skip past the rest of the heading line (in case of trailing whitespace)
	if nlIdx := strings.Index(content[start:], "\n"); nlIdx != -1 {
		start += nlIdx + 1
	} else {
		// Heading is the entire remaining content — no body
		return "", fmt.Errorf("acceptance criteria section is empty")
	}

	// Find the next ## heading
	rest := content[start:]
	endIdx := strings.Index(rest, "\n## ")
	var body string
	if endIdx == -1 {
		// AC section is the last section — take everything to EOF
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
