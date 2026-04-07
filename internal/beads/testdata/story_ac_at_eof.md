# Story 3.1: Final Section Story

Status: ready-for-dev

## Story

As an operator,
I want to test AC at end of file.

## Acceptance Criteria

1. **Given** the AC section is last
   **When** parsed
   **Then** content through EOF is captured

2. **Given** no trailing heading
   **When** the parser reaches EOF
   **Then** it still returns the AC content
