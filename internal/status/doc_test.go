package status_test

import (
	"fmt"
	"os"
	"path/filepath"

	"story-factory/internal/status"
)

// This example demonstrates using Reader to parse sprint status and classify entries.
func Example_reader() {
	// Create a temporary directory with a sample sprint-status.yaml
	tmpDir, err := os.MkdirTemp("", "status-reader")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	// Create the status file path structure
	statusDir := filepath.Join(tmpDir, "_bmad-output", "implementation-artifacts")
	if err := os.MkdirAll(statusDir, 0755); err != nil {
		fmt.Println("Error:", err)
		return
	}

	// Write a sample sprint-status.yaml
	statusYAML := `development_status:
  epic-1: in-progress
  1-1-define-schema: done
  1-2-add-api: ready-for-dev
  1-3-add-tests: backlog
  epic-1-retrospective: optional
`
	statusFile := filepath.Join(statusDir, "sprint-status.yaml")
	if err := os.WriteFile(statusFile, []byte(statusYAML), 0644); err != nil {
		fmt.Println("Error:", err)
		return
	}

	reader := status.NewReader(tmpDir)

	// Read returns all classified entries in YAML file order
	entries, err := reader.Read()
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Total entries:", len(entries))
	fmt.Println("First entry type:", entries[0].Type)

	// StoryByKey looks up a specific entry
	entry, err := reader.StoryByKey("1-2-add-api")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Story 1-2 status:", entry.Status)
	// Output:
	// Total entries: 5
	// First entry type: epic
	// Story 1-2 status: ready-for-dev
}

// This example demonstrates querying backlog stories and filtering by epic.
func Example_queries() {
	tmpDir, err := os.MkdirTemp("", "status-queries")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	statusDir := filepath.Join(tmpDir, "_bmad-output", "implementation-artifacts")
	if err := os.MkdirAll(statusDir, 0755); err != nil {
		fmt.Println("Error:", err)
		return
	}

	statusYAML := `development_status:
  epic-1: in-progress
  1-1-define-schema: done
  1-2-add-api: backlog
  epic-2: backlog
  2-1-setup-auth: backlog
`
	statusFile := filepath.Join(statusDir, "sprint-status.yaml")
	if err := os.WriteFile(statusFile, []byte(statusYAML), 0644); err != nil {
		fmt.Println("Error:", err)
		return
	}

	reader := status.NewReader(tmpDir)

	// StoriesByStatus returns stories matching any status string
	backlog, err := reader.StoriesByStatus("backlog")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Backlog stories:", len(backlog))

	// StoriesForEpic returns stories belonging to a specific epic
	epic1Stories, err := reader.StoriesForEpic(1)
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Epic 1 stories:", len(epic1Stories))
	// Output:
	// Backlog stories: 2
	// Epic 1 stories: 2
}

// This example demonstrates resolving the story file location path.
func Example_resolveStoryLocation() {
	tmpDir, err := os.MkdirTemp("", "status-resolve")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	statusDir := filepath.Join(tmpDir, "_bmad-output", "implementation-artifacts")
	if err := os.MkdirAll(statusDir, 0755); err != nil {
		fmt.Println("Error:", err)
		return
	}

	statusYAML := `story_location: "{project-root}/_bmad-output/implementation-artifacts"
development_status:
  1-1-define-schema: backlog
`
	statusFile := filepath.Join(statusDir, "sprint-status.yaml")
	if err := os.WriteFile(statusFile, []byte(statusYAML), 0644); err != nil {
		fmt.Println("Error:", err)
		return
	}

	reader := status.NewReader(tmpDir)

	resolved, err := reader.ResolveStoryLocation("/home/tom/projects/my-app")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}
	fmt.Println("Story location:", resolved)
	// Output:
	// Story location: /home/tom/projects/my-app/_bmad-output/implementation-artifacts
}
