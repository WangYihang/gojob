package gojob

import (
	"context"

	"github.com/google/uuid"
)

// Task is an interface that defines a task
type Task interface {
	// Do starts the task, returns error if failed
	// If an error is returned, the task will be retried until MaxRetries
	// You can set MaxRetries by calling SetMaxRetries on the scheduler
	Do(context.Context) error
}

type basicTask struct {
	Index      int64  `json:"index"`
	RunID      string `json:"run_id"`
	ID         string `json:"id"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
	NumTries   int    `json:"num_tries"`
	Task       Task   `json:"task"`
	Error      string `json:"error"`
}

func newBasicTask(index int64, runID string, task Task) *basicTask {
	return &basicTask{
		Index:      index,
		RunID:      runID,
		ID:         uuid.New().String(),
		StartedAt:  0,
		FinishedAt: 0,
		NumTries:   0,
		Task:       task,
		Error:      "",
	}
}
