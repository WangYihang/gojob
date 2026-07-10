package utils_test

import (
	"bytes"
	"testing"

	"github.com/WangYihang/gojob/pkg/utils"
)

type nopWriteCloser struct {
	buf *bytes.Buffer
}

func (n *nopWriteCloser) Write(p []byte) (int, error) { return n.buf.Write(p) }
func (n *nopWriteCloser) Close() error                { return nil }

func TestTeeWriterCloser(t *testing.T) {
	b1, b2 := &bytes.Buffer{}, &bytes.Buffer{}
	tee := utils.NewTeeWriterCloser(&nopWriteCloser{b1}, &nopWriteCloser{b2})

	n, err := tee.Write([]byte("hello"))
	if err != nil {
		t.Fatalf("Write returned error: %v", err)
	}
	if n != 5 {
		t.Errorf("Write: want n=5, got %d", n)
	}
	if err := tee.Close(); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
	if b1.String() != "hello" || b2.String() != "hello" {
		t.Errorf("Tee: want both writers to hold %q, got %q and %q", "hello", b1.String(), b2.String())
	}
}
