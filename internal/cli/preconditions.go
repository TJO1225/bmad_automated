package cli

import (
	"errors"
	"fmt"

	"story-factory/internal/pipeline"
)

// RunPreconditions verifies that all required dependencies are available
// before processing. On failure it prints the error via [App.Printer]
// and returns an [ExitError] with code 2 (FR31).
//
// This method should be called as the first step of every processing command.
func (app *App) RunPreconditions() error {
	projectDir, err := app.ResolveProjectDir()
	if err != nil {
		return fmt.Errorf("failed to determine working directory: %w", err)
	}

	check := func(dir string) error {
		return pipeline.CheckAll(dir, app.Mode)
	}
	if app.CheckPreconditions != nil {
		check = app.CheckPreconditions
	}

	if err := check(projectDir); err != nil {
		var precondErr *pipeline.PreconditionError
		if errors.As(err, &precondErr) {
			app.Printer.Text(fmt.Sprintf("Precondition check failed: %s", precondErr.Detail))
		} else {
			app.Printer.Text(fmt.Sprintf("Precondition check failed: %s", err))
		}
		return NewExitError(2)
	}

	return nil
}
