package output

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

// StepResult represents the result of a single workflow step execution.
//
// It captures the step name, execution duration, and success/failure status
// for display in cycle summaries.
type StepResult struct {
	// Name is the step identifier (e.g., "create-story", "dev-story").
	Name string
	// Duration is how long the step took to execute.
	Duration time.Duration
	// Success indicates whether the step completed successfully.
	Success bool
}

// StoryResult represents the result of processing a story in queue or epic operations.
//
// It tracks the outcome of each story in a batch operation, including whether
// it was skipped (already done), completed successfully, or failed.
type StoryResult struct {
	// Key is the story identifier (e.g., "7-1-define-schema").
	Key string
	// Success indicates whether the story completed all lifecycle steps.
	Success bool
	// Duration is how long the story processing took.
	Duration time.Duration
	// FailedAt contains the step name where processing failed, if any.
	FailedAt string
	// Reason is the failure description when the story failed.
	Reason string
	// Skipped indicates the story was skipped because it was already done.
	Skipped bool
	// NeedsReview indicates code-review flipped the story back to in-progress
	// because findings remain. Surfaced distinctly from ordinary failures.
	NeedsReview bool
	// BeadID is the bead identifier created during sync (beads mode only).
	BeadID string
	// PRURL is the pull-request URL opened by the open-pr step (Phase 2).
	PRURL string
	// StepsExecuted lists the step names that ran for this story, in order.
	StepsExecuted []string
}

// BatchCounts holds pre-computed counts from a batch operation.
//
// Counts are populated by pipeline batch methods from the Stories slice.
// StepCounts maps step name -> count of stories that completed that step
// successfully, so the totals line adapts to the active mode.
type BatchCounts struct {
	StepCounts  map[string]int
	Failed      int
	Skipped     int
	NeedsReview int
}

// Printer defines the interface for structured terminal output operations.
//
// The interface enables output capture in tests via [NewPrinterWithWriter],
// which accepts a custom io.Writer instead of writing to stdout.
//
// Methods are grouped by operation type: session lifecycle, step progress,
// tool usage display, content output, cycle summaries, and queue summaries.
type Printer interface {
	// SessionStart prints an indicator that a new execution session has begun.
	SessionStart()
	// SessionEnd prints completion status for the session with total duration.
	SessionEnd(duration time.Duration, success bool)

	// StepStart prints a numbered step header (e.g., "[1/4] create-story").
	StepStart(step, total int, name string)
	// StepEnd prints step completion status with duration.
	StepEnd(duration time.Duration, success bool)

	// ToolUse displays Claude tool invocation details including name,
	// description, command, and file path as applicable.
	ToolUse(name, description, command, filePath string)
	// ToolResult displays tool execution output, optionally truncating
	// stdout to the specified number of lines.
	ToolResult(stdout, stderr string, truncateLines int)

	// Text displays plain text content from Claude.
	Text(message string)
	// Divider prints a visual separator line between sections.
	Divider()

	// CycleHeader prints the header for a full lifecycle cycle operation.
	CycleHeader(storyKey string)
	// CycleSummary prints the completion summary showing all steps and durations.
	CycleSummary(storyKey string, steps []StepResult, totalDuration time.Duration)
	// CycleFailed prints failure information when a cycle fails at a step.
	CycleFailed(storyKey string, failedStep string, duration time.Duration)

	// QueueHeader prints the header for a batch queue operation.
	QueueHeader(count int, stories []string)
	// QueueStoryStart prints the header when starting a story in a queue.
	QueueStoryStart(index, total int, storyKey string)
	// QueueSummary prints a flat batch results summary with per-story
	// details (validation loops, bead IDs, failure reasons) and totals.
	QueueSummary(results []StoryResult, counts BatchCounts, totalDuration time.Duration)
	// BatchSummary prints an epic-grouped batch results summary with
	// per-epic headers, subtotals, and grand totals.
	BatchSummary(results []StoryResult, counts BatchCounts, totalDuration time.Duration)

	// CommandHeader prints the header before running a workflow command.
	CommandHeader(label, prompt string, truncateLength int)
	// CommandFooter prints the footer after a command completes with
	// duration, success status, and exit code.
	CommandFooter(duration time.Duration, success bool, exitCode int)
}

// DefaultPrinter implements [Printer] with lipgloss terminal styling.
//
// It is the production implementation used for CLI output. The styles
// are defined in styles.go and provide consistent color and formatting
// across all output operations.
type DefaultPrinter struct {
	out io.Writer
}

// NewPrinter creates a new [DefaultPrinter] that writes to stdout.
//
// This is the standard constructor for production CLI output.
func NewPrinter() *DefaultPrinter {
	return &DefaultPrinter{out: os.Stdout}
}

// NewPrinterWithWriter creates a new [DefaultPrinter] with a custom writer.
//
// This constructor enables output capture in tests by providing a bytes.Buffer
// or other io.Writer implementation instead of stdout.
func NewPrinterWithWriter(w io.Writer) *DefaultPrinter {
	return &DefaultPrinter{out: w}
}

func (p *DefaultPrinter) writeln(format string, args ...interface{}) {
	fmt.Fprintf(p.out, format+"\n", args...)
}

// SessionStart prints session start indicator.
func (p *DefaultPrinter) SessionStart() {
	p.writeln("%s Session started\n", iconInProgress)
}

// SessionEnd prints session end with status.
func (p *DefaultPrinter) SessionEnd(duration time.Duration, success bool) {
	statusIcon := iconError
	statusText := "Session failed"
	if success {
		statusIcon = iconSuccess
		statusText = "Session complete"
	}
	p.writeln("%s %s (%s)", statusIcon, statusText, duration.Round(time.Millisecond))
}

// StepStart prints step start header.
func (p *DefaultPrinter) StepStart(step, total int, name string) {
	header := fmt.Sprintf("[%d/%d] %s", step, total, name)
	p.writeln(stepHeaderStyle.Render(header))
}

// StepEnd prints step completion status.
func (p *DefaultPrinter) StepEnd(duration time.Duration, success bool) {
	statusIcon := iconError
	statusText := "failed"
	if success {
		statusIcon = iconSuccess
		statusText = "done"
	}
	p.writeln("%s Step %s (%s)", statusIcon, statusText, duration.Round(time.Millisecond))
}

// ToolUse prints tool invocation details.
func (p *DefaultPrinter) ToolUse(name, description, command, filePath string) {
	p.writeln("%s Tool: %s", iconTool, toolNameStyle.Render(name))

	if description != "" {
		p.writeln("%s  %s", iconToolLine, description)
	}
	if command != "" {
		p.writeln("%s  $ %s", iconToolLine, command)
	}
	if filePath != "" {
		p.writeln("%s  File: %s", iconToolLine, filePath)
	}

	p.writeln(iconToolEnd)
}

// ToolResult prints tool execution results.
func (p *DefaultPrinter) ToolResult(stdout, stderr string, truncateLines int) {
	if stdout != "" {
		output := truncateOutput(stdout, truncateLines)
		// Indent each line
		indented := "   " + strings.ReplaceAll(output, "\n", "\n   ")
		p.writeln("%s\n", indented)
	}
	if stderr != "" {
		p.writeln("   %s\n", mutedStyle.Render("[stderr] "+stderr))
	}
}

// Text prints a text message from Claude.
func (p *DefaultPrinter) Text(message string) {
	if message != "" {
		p.writeln("Claude: %s\n", message)
	}
}

// Divider prints a visual divider.
func (p *DefaultPrinter) Divider() {
	p.writeln(dividerStyle.Render(strings.Repeat("═", 65)))
}

// CycleHeader prints the header for a full cycle run.
func (p *DefaultPrinter) CycleHeader(storyKey string) {
	p.writeln("")
	content := fmt.Sprintf("Story Factory: %s", storyKey)
	p.writeln(headerStyle.Render(content))
	p.writeln("")
}

// CycleSummary prints the summary after a successful cycle.
func (p *DefaultPrinter) CycleSummary(storyKey string, steps []StepResult, totalDuration time.Duration) {
	var sb strings.Builder

	sb.WriteString(successStyle.Render(iconSuccess+" CYCLE COMPLETE") + "\n")
	sb.WriteString(fmt.Sprintf("Story: %s\n", storyKey))
	sb.WriteString(strings.Repeat("─", 50) + "\n")

	for i, step := range steps {
		sb.WriteString(fmt.Sprintf("[%d] %-15s %s\n", i+1, step.Name, step.Duration.Round(time.Millisecond)))
	}

	sb.WriteString(strings.Repeat("─", 50) + "\n")
	sb.WriteString(fmt.Sprintf("Total: %s", totalDuration.Round(time.Millisecond)))

	p.writeln(summaryStyle.Render(sb.String()))
}

// CycleFailed prints failure information when a cycle fails.
func (p *DefaultPrinter) CycleFailed(storyKey string, failedStep string, duration time.Duration) {
	var sb strings.Builder

	sb.WriteString(errorStyle.Render(iconError+" CYCLE FAILED") + "\n")
	sb.WriteString(fmt.Sprintf("Story: %s\n", storyKey))
	sb.WriteString(fmt.Sprintf("Failed at: %s\n", failedStep))
	sb.WriteString(fmt.Sprintf("Duration: %s", duration.Round(time.Millisecond)))

	p.writeln(summaryStyle.Render(sb.String()))
}

// QueueHeader prints the header for a queue run.
func (p *DefaultPrinter) QueueHeader(count int, stories []string) {
	p.writeln("")
	storiesList := truncateString(strings.Join(stories, ", "), 50)
	content := fmt.Sprintf("BMAD Queue: %d stories\nStories: %s", count, storiesList)
	p.writeln(headerStyle.Render(content))
	p.writeln("")
}

// QueueStoryStart prints the header for starting a story in a queue.
func (p *DefaultPrinter) QueueStoryStart(index, total int, storyKey string) {
	header := fmt.Sprintf("QUEUE [%d/%d]: %s", index, total, storyKey)
	p.writeln(queueHeaderStyle.Render(header))
}

// QueueSummary prints a flat batch summary with per-story details.
func (p *DefaultPrinter) QueueSummary(results []StoryResult, counts BatchCounts, totalDuration time.Duration) {
	var sb strings.Builder

	if counts.Failed == 0 {
		sb.WriteString(successStyle.Render(iconSuccess+" BATCH COMPLETE") + "\n")
	} else {
		sb.WriteString(errorStyle.Render(iconError+" BATCH FAILED") + "\n")
	}

	sb.WriteString(strings.Repeat("─", 50) + "\n")

	for _, r := range results {
		sb.WriteString(formatStoryRow(r) + "\n")
	}

	sb.WriteString(strings.Repeat("─", 50) + "\n")
	sb.WriteString(formatTotals(counts) + "\n")
	sb.WriteString(fmt.Sprintf("Total: %s", totalDuration.Round(time.Second)))

	p.writeln(summaryStyle.Render(sb.String()))
}

// BatchSummary prints an epic-grouped batch summary with per-epic subtotals.
func (p *DefaultPrinter) BatchSummary(results []StoryResult, counts BatchCounts, totalDuration time.Duration) {
	var sb strings.Builder

	if counts.Failed == 0 {
		sb.WriteString(successStyle.Render(iconSuccess+" QUEUE COMPLETE") + "\n")
	} else {
		sb.WriteString(errorStyle.Render(iconError+" QUEUE FAILED") + "\n")
	}

	// Group stories by epic number (preserves input order)
	type epicGroup struct {
		num     int
		stories []StoryResult
	}
	var groups []epicGroup
	seen := make(map[int]int)

	for _, r := range results {
		num := epicNumFromKey(r.Key)
		if idx, ok := seen[num]; ok {
			groups[idx].stories = append(groups[idx].stories, r)
		} else {
			seen[num] = len(groups)
			groups = append(groups, epicGroup{num: num, stories: []StoryResult{r}})
		}
	}

	for _, g := range groups {
		sb.WriteString(fmt.Sprintf("\n%s\n",
			labelStyle.Render(fmt.Sprintf("── Epic %d ──────────────────────────────────", g.num))))

		created := 0
		failed := 0
		skipped := 0
		for _, r := range g.stories {
			sb.WriteString(formatStoryRow(r) + "\n")
			if r.Skipped {
				skipped++
			} else if r.Success {
				created++
			} else {
				failed++
			}
		}

		subtotal := fmt.Sprintf("  Epic %d: %d created, %d failed", g.num, created, failed)
		if skipped > 0 {
			subtotal += fmt.Sprintf(", %d skipped", skipped)
		}
		sb.WriteString(mutedStyle.Render(subtotal) + "\n")
	}

	sb.WriteString("\n" + strings.Repeat("─", 50) + "\n")
	sb.WriteString(formatTotals(counts) + "\n")
	sb.WriteString(fmt.Sprintf("Total: %s", totalDuration.Round(time.Second)))

	p.writeln(summaryStyle.Render(sb.String()))
}

// formatStoryRow renders a single story result row with status icon and details.
func formatStoryRow(r StoryResult) string {
	if r.Skipped {
		return fmt.Sprintf("%s %-30s  %s", mutedStyle.Render("↷"), r.Key, mutedStyle.Render("(skipped)"))
	}
	if r.Success {
		detail := fmt.Sprintf("steps:%d  %s", len(r.StepsExecuted), r.Duration.Round(time.Second))
		if r.BeadID != "" {
			detail = fmt.Sprintf("bead:%s  %s", r.BeadID, detail)
		}
		if r.PRURL != "" {
			detail = fmt.Sprintf("pr:%s  %s", r.PRURL, detail)
		}
		return fmt.Sprintf("%s %-30s  %s", successStyle.Render(iconSuccess), r.Key, detail)
	}
	if r.NeedsReview {
		detail := fmt.Sprintf("needs-review at %s  %s", r.FailedAt, r.Duration.Round(time.Second))
		return fmt.Sprintf("%s %-30s  %s", labelStyle.Render("!"), r.Key, detail)
	}
	details := fmt.Sprintf("%s: %s  %s", r.FailedAt, r.Reason, r.Duration.Round(time.Second))
	return fmt.Sprintf("%s %-30s  %s", errorStyle.Render(iconError), r.Key, details)
}

// formatTotals renders the batch counts totals line. Step counts are shown
// in a stable alphabetical order so the output is deterministic regardless
// of map iteration order.
func formatTotals(c BatchCounts) string {
	var parts []string
	names := make([]string, 0, len(c.StepCounts))
	for name := range c.StepCounts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s: %d", name, c.StepCounts[name]))
	}
	parts = append(parts, fmt.Sprintf("Failed: %d", c.Failed))
	parts = append(parts, fmt.Sprintf("Skip: %d", c.Skipped))
	if c.NeedsReview > 0 {
		parts = append(parts, fmt.Sprintf("NeedsReview: %d", c.NeedsReview))
	}
	return strings.Join(parts, " | ")
}

// epicNumFromKey extracts the epic number from a story key (e.g., "3-1-slug" → 3).
// Returns 0 if the key prefix is not a valid integer.
func epicNumFromKey(key string) int {
	parts := strings.SplitN(key, "-", 3)
	n, _ := strconv.Atoi(parts[0])
	return n
}

// CommandHeader prints the header before running a command.
func (p *DefaultPrinter) CommandHeader(label, prompt string, truncateLength int) {
	p.Divider()
	p.writeln("  Command: %s", labelStyle.Render(label))
	p.writeln("  Prompt:  %s", truncateString(prompt, truncateLength))
	p.Divider()
	p.writeln("")
}

// CommandFooter prints the footer after a command completes.
func (p *DefaultPrinter) CommandFooter(duration time.Duration, success bool, exitCode int) {
	p.writeln("")
	p.Divider()
	if success {
		p.writeln("  %s | Duration: %s", successStyle.Render(iconSuccess+" SUCCESS"), duration.Round(time.Millisecond))
	} else {
		p.writeln("  %s | Duration: %s | Exit code: %d", errorStyle.Render(iconError+" FAILED"), duration.Round(time.Millisecond), exitCode)
	}
	p.Divider()
}

// truncateString truncates a string to maxLen, adding "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// truncateOutput truncates output to maxLines, showing first and last portions.
func truncateOutput(output string, maxLines int) string {
	if maxLines <= 0 {
		return output
	}

	lines := strings.Split(output, "\n")
	if len(lines) <= maxLines {
		return output
	}

	half := maxLines / 2
	omitted := len(lines) - maxLines

	first := strings.Join(lines[:half], "\n")
	last := strings.Join(lines[len(lines)-half:], "\n")

	return fmt.Sprintf("%s\n  ... (%d lines omitted) ...\n%s", first, omitted, last)
}
