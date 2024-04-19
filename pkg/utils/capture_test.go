package utils_test

import (
	"fmt"
	"testing"

	"github.com/WangYihang/gojob/pkg/utils"
)

func TestOutputCapture(t *testing.T) {
	oc := utils.NewStdoutCapture()
	oc.StartCapture()
	fmt.Println("hello")
	oc.StopCapture()
	if oc.GetCapturedOutput() != "hello\n" {
		t.Fail()
	}
}
