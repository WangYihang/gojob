package utils

import (
	"bytes"
	"io"
	"os"
)

type StdoutCapture struct {
	originalStdout *os.File
	r              *os.File
	w              *os.File
	buffer         bytes.Buffer
}

func NewStdoutCapture() *StdoutCapture {
	return &StdoutCapture{}
}

func (oc *StdoutCapture) StartCapture() {
	oc.originalStdout = os.Stdout

	r, w, _ := os.Pipe()
	os.Stdout = w

	oc.r = r
	oc.w = w
}

func (oc *StdoutCapture) StopCapture() {
	os.Stdout = oc.originalStdout
	oc.w.Close()

	io.Copy(&oc.buffer, oc.r)
	oc.r.Close()
}

func (oc *StdoutCapture) GetCapturedOutput() string {
	return oc.buffer.String()
}
