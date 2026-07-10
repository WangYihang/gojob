package utils_test

import (
	"context"
	"testing"
	"time"

	"github.com/WangYihang/gojob/pkg/utils"
)

type Task struct{}

func (t *Task) Do(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(16 * time.Second):
		return nil
	}
}

func TestRunWithTimeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	task := &Task{}
	err := utils.RunWithTimeout(ctx, task.Do)
	if err == nil {
		t.Errorf("Expected timeout error, got nil")
	}
}
