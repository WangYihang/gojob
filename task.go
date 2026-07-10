package gojob

import (
	"context"
	"encoding/json"
	"time"
)

// Task is a self-contained unit of work that produces a typed result.
// Use it with Execute when you prefer "one task object per item" over a plain
// mapping function.
type Task[T any] interface {
	Execute(ctx context.Context) (T, error)
}

// TaskFunc adapts a plain function to a Task.
type TaskFunc[T any] func(ctx context.Context) (T, error)

// Execute implements Task.
func (f TaskFunc[T]) Execute(ctx context.Context) (T, error) { return f(ctx) }

// Result is the outcome of processing a single item, carrying the produced
// value alongside its error and execution metadata.
type Result[T any] struct {
	Value     T
	Err       error
	Attempts  int
	StartedAt time.Time
	Duration  time.Duration
}

// MarshalJSON renders a Result as a flat, log-friendly JSON object. The error
// is emitted as a string ("" when there was none), so results serialize cleanly
// to JSON Lines.
func (r Result[T]) MarshalJSON() ([]byte, error) {
	var errStr string
	if r.Err != nil {
		errStr = r.Err.Error()
	}
	return json.Marshal(struct {
		Value      T      `json:"value"`
		Error      string `json:"error"`
		Attempts   int    `json:"attempts"`
		StartedAt  int64  `json:"started_at"`
		DurationMs int64  `json:"duration_ms"`
	}{
		Value:      r.Value,
		Error:      errStr,
		Attempts:   r.Attempts,
		StartedAt:  r.StartedAt.UnixMicro(),
		DurationMs: r.Duration.Milliseconds(),
	})
}
