package utils_test

import (
	"context"
	"testing"
	"time"

	"github.com/WangYihang/gojob/pkg/utils"
)

func TestRunWithTimeout(t *testing.T) {
	task := func(_ context.Context) error {
		time.Sleep(2 * time.Second)
		return nil
	}

	err := utils.RunWithTimeout(task, 1*time.Second)
	if err == nil {
		t.Errorf("Expected timeout error, got nil")
	}
}
