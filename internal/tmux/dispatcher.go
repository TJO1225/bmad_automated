package tmux

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"story-factory/internal/git"
)

// DispatchResult captures the outcome for a single story dispatched through
// the tmux supervisor.
type DispatchResult struct {
	Key          string
	WorktreePath string
	Branch       string
	ExitCode     int
	Err          error
	Duration     time.Duration
}

// BatchSummary aggregates DispatchResults for display at the end of a run.
type BatchSummary struct {
	Results     []DispatchResult
	Duration    time.Duration
	Succeeded   int
	Failed      int
	WorktreeDir string
}

// DispatcherOptions configures a Dispatcher. All fields are required unless
// marked optional.
type DispatcherOptions struct {
	// Parallel is the maximum number of concurrent stories. The supervisor
	// creates ceil(Parallel/4) windows each with up to 4 panes.
	Parallel int
	// ProjectRoot is the absolute path to the main repository.
	ProjectRoot string
	// WorktreeRoot is where per-story worktrees are created
	// (e.g. <project>/.story-factory/worktrees).
	WorktreeRoot string
	// BaseBranch is the branch new worktrees are forked from (typically
	// "main" or "master").
	BaseBranch string
	// BinaryPath is the absolute path to the story-factory binary the
	// supervisor tells each pane to run. Use os.Executable() so panes run
	// the same build as the supervisor.
	BinaryPath string
	// Mode forwarded to `story-factory run` in each pane (typically "bmad").
	Mode string
	// Verbose passes --verbose to the dispatched commands.
	Verbose bool
	// PollInterval is how often the supervisor polls panes for the
	// completion sentinel. Optional; defaults to 2s.
	PollInterval time.Duration
	// Scrollback is how many lines of scrollback to capture per poll.
	// Optional; defaults to 200.
	Scrollback int
}

// Dispatcher orchestrates the parallel tmux workflow.
type Dispatcher struct {
	opts    DispatcherOptions
	panes   []string // pane IDs in slot order
	mu      sync.Mutex
	results []DispatchResult
	logger  func(string)
}

// NewDispatcher validates opts and returns a Dispatcher ready to Run.
func NewDispatcher(opts DispatcherOptions, logger func(string)) (*Dispatcher, error) {
	if opts.Parallel <= 0 {
		return nil, errors.New("parallel must be > 0")
	}
	if opts.ProjectRoot == "" {
		return nil, errors.New("ProjectRoot is required")
	}
	if opts.WorktreeRoot == "" {
		return nil, errors.New("WorktreeRoot is required")
	}
	if opts.BaseBranch == "" {
		return nil, errors.New("BaseBranch is required")
	}
	if opts.BinaryPath == "" {
		return nil, errors.New("BinaryPath is required")
	}
	if opts.Mode == "" {
		opts.Mode = "bmad"
	}
	if opts.PollInterval == 0 {
		opts.PollInterval = 2 * time.Second
	}
	if opts.Scrollback == 0 {
		opts.Scrollback = 200
	}
	if logger == nil {
		logger = func(string) {}
	}
	return &Dispatcher{opts: opts, logger: logger}, nil
}

// Run dispatches each story key across the configured slots. It blocks
// until all stories finish or ctx is canceled; on cancel, already-dispatched
// commands keep running in their panes (no subprocess interruption) but no
// new dispatches are made. Returns a summary with per-story outcomes.
func (d *Dispatcher) Run(ctx context.Context, stories []string) (BatchSummary, error) {
	start := time.Now()

	if !InSession() {
		return BatchSummary{}, errors.New("not inside a tmux session — run from a tmux pane or window")
	}
	if len(stories) == 0 {
		return BatchSummary{Duration: time.Since(start)}, nil
	}

	// Ensure worktree root exists.
	if err := os.MkdirAll(d.opts.WorktreeRoot, 0o755); err != nil {
		return BatchSummary{}, fmt.Errorf("create worktree root: %w", err)
	}

	// Cap parallelism at the number of stories — no point spawning idle panes.
	slots := d.opts.Parallel
	if slots > len(stories) {
		slots = len(stories)
	}

	// Create windows + panes.
	if err := d.layoutPanes(ctx, slots); err != nil {
		return BatchSummary{}, err
	}

	// Queue the work. Buffered so the sender goroutine never blocks.
	queue := make(chan string, len(stories))
	for _, k := range stories {
		queue <- k
	}
	close(queue)

	// Launch workers.
	var wg sync.WaitGroup
	for i := 0; i < slots; i++ {
		wg.Add(1)
		go d.worker(ctx, i, queue, &wg)
	}
	wg.Wait()

	return d.summarize(start), nil
}

// layoutPanes creates ceil(slots/4) tmux windows, splits each into up to
// 4 panes, applies a tiled layout, and records pane IDs in d.panes in slot
// order (window 1 panes 1-4, window 2 panes 1-4, ...).
func (d *Dispatcher) layoutPanes(ctx context.Context, slots int) error {
	windows := (slots + 3) / 4 // ceil
	d.panes = make([]string, 0, slots)

	for w := 0; w < windows; w++ {
		paneCount := 4
		if remaining := slots - w*4; remaining < 4 {
			paneCount = remaining
		}
		windowName := fmt.Sprintf("sf-batch-%d", w+1)
		firstPane, err := NewWindow(ctx, windowName)
		if err != nil {
			return fmt.Errorf("create tmux window %s: %w", windowName, err)
		}
		windowPanes := []string{firstPane}
		for p := 1; p < paneCount; p++ {
			newPane, err := SplitPane(ctx, firstPane, "v")
			if err != nil {
				return fmt.Errorf("split pane in %s: %w", windowName, err)
			}
			windowPanes = append(windowPanes, newPane)
		}
		if paneCount > 1 {
			if err := SelectLayoutTiled(ctx, firstPane); err != nil {
				return fmt.Errorf("tile layout in %s: %w", windowName, err)
			}
		}
		_ = EnablePaneBorderStatus(ctx) // best-effort — border status is cosmetic
		d.panes = append(d.panes, windowPanes...)
	}
	return nil
}

// worker is one supervisor goroutine. It loops pulling story keys from the
// shared queue and running each through its assigned pane. When the queue
// closes and is drained, the goroutine exits.
func (d *Dispatcher) worker(ctx context.Context, slot int, queue <-chan string, wg *sync.WaitGroup) {
	defer wg.Done()
	paneID := d.panes[slot]
	for {
		select {
		case <-ctx.Done():
			d.logger(fmt.Sprintf("[slot %d] canceled", slot))
			return
		case key, ok := <-queue:
			if !ok {
				return
			}
			result := d.runStory(ctx, slot, paneID, key)
			d.recordResult(result)
		}
	}
}

// runStory dispatches a single story to paneID and waits for completion.
func (d *Dispatcher) runStory(ctx context.Context, slot int, paneID, key string) DispatchResult {
	start := time.Now()
	result := DispatchResult{Key: key}

	// Label the pane so the user can tell at a glance what's where.
	_ = SetPaneTitle(ctx, paneID, fmt.Sprintf("[%d] %s", slot+1, key))

	branch := "story/" + key
	worktreePath := filepath.Join(d.opts.WorktreeRoot, key)
	result.Branch = branch
	result.WorktreePath = worktreePath

	// Resume: if the worktree already exists from a prior run, reuse it.
	// Otherwise create a fresh one on a new story branch.
	if _, err := os.Stat(worktreePath); err == nil {
		d.logger(fmt.Sprintf("[slot %d] reusing existing worktree %s", slot, worktreePath))
	} else if os.IsNotExist(err) {
		if err := git.AddWorktree(ctx, d.opts.ProjectRoot, worktreePath, branch, d.opts.BaseBranch); err != nil {
			result.Err = fmt.Errorf("create worktree: %w", err)
			result.Duration = time.Since(start)
			return result
		}
	} else {
		result.Err = fmt.Errorf("stat worktree path: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Build the shell command to run in the pane. The sentinel appended at
	// the end includes a per-dispatch nonce so we can distinguish this run's
	// completion from any previous one still in the pane scrollback.
	nonce := newNonce()
	verboseFlag := ""
	if d.opts.Verbose {
		verboseFlag = " --verbose"
	}
	cmd := fmt.Sprintf(
		"cd %s && %s run %s --mode=%s --project-dir %s%s; echo \"__SF_DONE__:%s:$?\"",
		shellQuote(worktreePath),
		shellQuote(d.opts.BinaryPath),
		shellQuote(key),
		shellQuote(d.opts.Mode),
		shellQuote(worktreePath),
		verboseFlag,
		nonce,
	)

	if err := SendKeys(ctx, paneID, cmd); err != nil {
		result.Err = fmt.Errorf("send keys to pane %s: %w", paneID, err)
		result.Duration = time.Since(start)
		return result
	}

	exitCode, err := d.waitForSentinel(ctx, paneID, nonce)
	result.ExitCode = exitCode
	result.Err = err
	result.Duration = time.Since(start)
	d.logger(fmt.Sprintf("[slot %d] %s finished exit=%d", slot, key, exitCode))
	return result
}

// waitForSentinel polls the given pane until it sees the completion sentinel
// for nonce and returns the captured exit code. Blocks until ctx is done.
func (d *Dispatcher) waitForSentinel(ctx context.Context, paneID, nonce string) (int, error) {
	ticker := time.NewTicker(d.opts.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return -1, ctx.Err()
		case <-ticker.C:
			content, err := CapturePane(ctx, paneID, d.opts.Scrollback)
			if err != nil {
				return -1, fmt.Errorf("capture pane %s: %w", paneID, err)
			}
			if exit, ok := ParseSentinel(content, nonce); ok {
				return exit, nil
			}
		}
	}
}

// sentinelPattern matches "__SF_DONE__:<nonce>:<exit>" with a dynamic nonce.
// Built per-call rather than global so different dispatches don't cross-fire.
func sentinelPattern(nonce string) *regexp.Regexp {
	// nonce is hex, so no regex metachars — plain concatenation is safe.
	return regexp.MustCompile(`__SF_DONE__:` + regexp.QuoteMeta(nonce) + `:(\d+)`)
}

// ParseSentinel scans captured pane content for the completion marker
// matching nonce. Returns (exit, true) if found, (0, false) otherwise.
// Exported so unit tests can verify the parser without a live tmux.
func ParseSentinel(captured, nonce string) (int, bool) {
	m := sentinelPattern(nonce).FindStringSubmatch(captured)
	if m == nil {
		return 0, false
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return n, true
}

// recordResult appends a dispatch result to the shared slice under a mutex.
func (d *Dispatcher) recordResult(r DispatchResult) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.results = append(d.results, r)
}

// summarize converts the collected results into a BatchSummary.
func (d *Dispatcher) summarize(start time.Time) BatchSummary {
	d.mu.Lock()
	defer d.mu.Unlock()
	sum := BatchSummary{
		Results:     append([]DispatchResult(nil), d.results...),
		Duration:    time.Since(start),
		WorktreeDir: d.opts.WorktreeRoot,
	}
	for _, r := range sum.Results {
		if r.Err == nil && r.ExitCode == 0 {
			sum.Succeeded++
		} else {
			sum.Failed++
		}
	}
	return sum
}

// newNonce returns a 16-char hex nonce used to uniquely identify a single
// dispatch in pane scrollback.
func newNonce() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// shellQuote wraps s in single quotes with embedded quotes escaped, safe to
// splice into a bash command line. Used so paths with spaces or special
// characters don't break the send-keys command.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
