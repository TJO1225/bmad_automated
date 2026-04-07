package pipeline

import (
	"errors"
	"fmt"
)

// ErrPreconditionFailed is a sentinel error indicating a precondition check failed.
// Use [errors.Is] to check if an error is a precondition failure.
var ErrPreconditionFailed = errors.New("precondition failed")

// PreconditionError describes which precondition check failed and why.
// It wraps [ErrPreconditionFailed] so callers can use [errors.Is] to detect
// precondition failures generically.
type PreconditionError struct {
	// Check identifies which precondition failed (e.g., "bd-cli", "sprint-status", "bmad-agents").
	Check string
	// Detail is a human-readable explanation of the failure.
	Detail string
}

// Error implements the error interface.
func (e *PreconditionError) Error() string {
	return fmt.Sprintf("precondition failed: %s: %s", e.Check, e.Detail)
}

// Unwrap returns [ErrPreconditionFailed] so [errors.Is] works.
func (e *PreconditionError) Unwrap() error {
	return ErrPreconditionFailed
}
