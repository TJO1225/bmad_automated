package pipeline

import (
	"bytes"
	"io"
)

// printerLineWriter adapts [Printer.Text] to [io.Writer], emitting one Text call
// per newline-terminated line. Call [printerLineWriter.flush] after the writer is
// done to print any trailing bytes without a final newline.
type printerLineWriter struct {
	p   Printer
	buf []byte
}

func newPrinterLineWriter(p Printer) *printerLineWriter {
	return &printerLineWriter{p: p}
}

func (w *printerLineWriter) Write(p []byte) (n int, err error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		line := string(w.buf[:i])
		w.buf = w.buf[i+1:]
		if line != "" {
			w.p.Text(line)
		}
	}
	return len(p), nil
}

func (w *printerLineWriter) flush() {
	if len(w.buf) == 0 {
		return
	}
	s := string(w.buf)
	w.buf = w.buf[:0]
	if s != "" {
		w.p.Text(s)
	}
}

var _ io.Writer = (*printerLineWriter)(nil)
