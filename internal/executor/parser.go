package executor

import (
	"bufio"
	"io"
)

// LineScanner reads newline-delimited text from a reader, yielding one line at
// a time. Backend parsers use this to iterate over streaming JSON output.
//
// The BufferSize field controls the maximum allowed line length, which is
// important because LLM CLIs may output large JSON objects (e.g., file
// contents in tool results).
type LineScanner struct {
	// BufferSize is the maximum size in bytes for a single line.
	// Defaults to 10MB if not set or <= 0.
	BufferSize int
}

// NewLineScanner creates a [LineScanner] with the default 10MB buffer.
func NewLineScanner() *LineScanner {
	return &LineScanner{BufferSize: 10 * 1024 * 1024}
}

// Scan reads lines from the reader and sends each non-empty line to the
// returned channel. The channel is closed when the reader is exhausted.
func (s *LineScanner) Scan(reader io.Reader) <-chan string {
	lines := make(chan string)

	go func() {
		defer close(lines)

		scanner := bufio.NewScanner(reader)

		bufSize := s.BufferSize
		if bufSize <= 0 {
			bufSize = 10 * 1024 * 1024
		}
		buf := make([]byte, 0, 1024*1024)
		scanner.Buffer(buf, bufSize)

		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			lines <- line
		}
	}()

	return lines
}
