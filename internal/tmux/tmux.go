// Package tmux is a thin wrapper over the tmux CLI used by the story-factory
// dispatcher to drive a multi-pane tmux layout.
//
// The package intentionally avoids a tmux client library — shelling out
// keeps the binary small and matches what a user would do from their own
// keyboard, which makes the layout predictable and easy to debug.
//
// Functions return the new identifiers tmux prints with `-P -F` so callers
// can address each pane individually. All functions are safe to call
// concurrently because every tmux invocation is its own subprocess.
package tmux

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// InSession reports whether the current process is running inside tmux.
// The dispatcher refuses to run outside tmux because it would otherwise
// have nowhere to draw its panes.
func InSession() bool {
	return os.Getenv("TMUX") != ""
}

// runTmux runs `tmux <args...>` and returns trimmed stdout. Stderr on
// failure is folded into the returned error.
func runTmux(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "tmux", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("tmux %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}

// NewWindow creates a new window in the current tmux session with the given
// name and returns the first pane's ID (e.g. "%12"). The new window is not
// made current so the caller's current window stays focused.
//
// The shell in the new pane is not altered — commands are issued via
// [SendKeys] afterwards.
func NewWindow(ctx context.Context, name string) (paneID string, err error) {
	return runTmux(ctx, "new-window", "-d", "-n", name, "-P", "-F", "#{pane_id}")
}

// SplitPane splits the given pane in the specified direction ("h" horizontal
// or "v" vertical) and returns the new pane's ID.
func SplitPane(ctx context.Context, targetPane, direction string) (paneID string, err error) {
	flag := "-h"
	if direction == "v" {
		flag = "-v"
	}
	return runTmux(ctx, "split-window", flag, "-t", targetPane, "-P", "-F", "#{pane_id}")
}

// SelectLayoutTiled applies the "tiled" layout to the window containing
// targetPane, producing a balanced grid. Used after creating N splits to
// arrange them as a 2×2 (or similar) grid.
func SelectLayoutTiled(ctx context.Context, targetPane string) error {
	_, err := runTmux(ctx, "select-layout", "-t", targetPane, "tiled")
	return err
}

// SendKeys types cmd into targetPane and presses Enter, submitting the
// command to the pane's shell.
//
// The command is passed verbatim; the caller is responsible for any shell
// escaping needed.
func SendKeys(ctx context.Context, targetPane, cmd string) error {
	_, err := runTmux(ctx, "send-keys", "-t", targetPane, cmd, "Enter")
	return err
}

// CapturePane returns the visible contents of targetPane plus the last
// `scrollback` lines of history. Used by the dispatcher to poll for the
// completion sentinel after SendKeys.
func CapturePane(ctx context.Context, targetPane string, scrollback int) (string, error) {
	args := []string{"capture-pane", "-p", "-t", targetPane}
	if scrollback > 0 {
		args = append(args, "-S", fmt.Sprintf("-%d", scrollback))
	}
	return runTmux(ctx, args...)
}

// SetPaneTitle sets the pane's title attribute (visible in the pane border
// when `pane-border-status` is enabled). The dispatcher labels each pane
// with the story key it's currently running.
func SetPaneTitle(ctx context.Context, targetPane, title string) error {
	_, err := runTmux(ctx, "select-pane", "-t", targetPane, "-T", title)
	return err
}

// KillPane kills the given pane. Used in cleanup paths if the dispatcher
// decides to close panes instead of reusing them.
func KillPane(ctx context.Context, targetPane string) error {
	_, err := runTmux(ctx, "kill-pane", "-t", targetPane)
	return err
}

// EnablePaneBorderStatus turns on per-pane titles for the current window so
// the dispatcher's pane labels are visible.
func EnablePaneBorderStatus(ctx context.Context) error {
	_, err := runTmux(ctx, "set-option", "-w", "pane-border-status", "top")
	return err
}
