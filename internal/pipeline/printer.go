package pipeline

import "time"

// Printer is the interface for pipeline step output.
//
// This interface is defined in pipeline/ and satisfied implicitly by
// output.DefaultPrinter. Pipeline code depends on this interface, not
// on the output package directly.
type Printer interface {
	Text(message string)
	StepStart(step, total int, name string)
	StepEnd(duration time.Duration, success bool)
}
