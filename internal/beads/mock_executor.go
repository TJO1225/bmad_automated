package beads

import (
	"context"
	"fmt"
	"io"
)

// CreateCall records the arguments passed to a single [MockExecutor.Create] call.
type CreateCall struct {
	Key   string
	Title string
	ACs   string
}

// MockExecutor implements [Executor] for testing without a real bd binary.
//
// Configure the mock by setting its fields before calling Create:
//
//	mock := &MockExecutor{BeadID: "bd-abc123"}
//	id, err := mock.Create(ctx, "1-2-db-schema", "Database Schema", "ACs...", nil)
//
// After execution, check Calls to verify the arguments that were passed.
type MockExecutor struct {
	// BeadID is returned from Create on success.
	BeadID string

	// Err is returned from Create if non-nil.
	Err error

	// Calls records all Create invocations for assertion.
	Calls []CreateCall
}

// Create records the call and returns the pre-configured [MockExecutor.BeadID]
// and [MockExecutor.Err].
func (m *MockExecutor) Create(_ context.Context, key, title, acs string, bdOut io.Writer) (string, error) {
	m.Calls = append(m.Calls, CreateCall{Key: key, Title: title, ACs: acs})
	if bdOut != nil {
		_, _ = fmt.Fprintf(bdOut, "mock bd stdout for %s\n", key)
	}
	if m.Err != nil {
		return "", m.Err
	}
	return m.BeadID, nil
}
