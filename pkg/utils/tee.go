package utils

import (
	"io"
)

// TeeWriterCloser structure, used to write to and close multiple io.WriteClosers
type TeeWriterCloser struct {
	writers []io.WriteCloser
}

// NewTeeWriterCloser creates a new instance of TeeWriterCloser
func NewTeeWriterCloser(writers ...io.WriteCloser) *TeeWriterCloser {
	return &TeeWriterCloser{
		writers: writers,
	}
}

// Write implements the io.Writer interface
// It writes the given byte slice to all the contained writers
func (t *TeeWriterCloser) Write(p []byte) (n int, err error) {
	for _, w := range t.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
	}
	return len(p), nil
}

// Close implements the io.Closer interface
// It closes all the contained writers and returns the first encountered error
func (t *TeeWriterCloser) Close() (err error) {
	for _, w := range t.writers {
		if cerr := w.Close(); cerr != nil && err == nil {
			err = cerr // Record the first encountered error
		}
	}
	return
}
