package utils_test

import (
	"context"
	"testing"
	"time"

	"github.com/WangYihang/gojob/pkg/utils"
)

type Task struct {
	ctx context.Context
}

func (t *Task) Do() error {
	time.Sleep(16 * time.Second)
	return nil
}

func TestRunWithTimeout(t *testing.T) {
	task := &Task{context.Background()}
	err := utils.RunWithTimeout(task.Do, 1*time.Second)
	if err == nil {
		t.Errorf("Expected timeout error, got nil")
	}
}
