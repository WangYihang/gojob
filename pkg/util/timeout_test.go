package util_test

import (
	"testing"
	"time"

	"github.com/WangYihang/gojob/pkg/util"
)

func TestRunWithTimeout(t *testing.T) {
	task := func() error {
		time.Sleep(2 * time.Second)
		return nil
	}

	err := util.RunWithTimeout(task, 1*time.Second)
	if err == nil {
		t.Errorf("Expected timeout error, got nil")
	}
}
