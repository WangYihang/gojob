package utils

import (
	"bytes"
	"io"
	"os"
)

type OutputCapture struct {
	originalStdout *os.File
	r              *os.File
	w              *os.File
	buffer         bytes.Buffer
}

func NewOutputCapture() *OutputCapture {
	return &OutputCapture{}
}

func (oc *OutputCapture) StartCapture() {
	oc.originalStdout = os.Stdout

	r, w, _ := os.Pipe()
	os.Stdout = w

	oc.r = r
	oc.w = w
}

func (oc *OutputCapture) StopCapture() {
	os.Stdout = oc.originalStdout
	oc.w.Close()

	io.Copy(&oc.buffer, oc.r)
	oc.r.Close()
}

func (oc *OutputCapture) GetCapturedOutput() string {
	return oc.buffer.String()
}

// capture := util.NewOutputCapture()
// capture.StartCapture()
// capture.StopCapture()
