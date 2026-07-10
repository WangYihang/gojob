// Command tasks shows the "one self-contained task object per item" style using
// Execute and the Task[T] interface — the closest analogue to the classic
// implement-an-interface pattern, but type-safe and context-aware.
package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/WangYihang/gojob"
)

// PingTask carries both its input (Host) and its output (Latency), and returns
// itself so the enriched value flows downstream as Result.Value.
type PingTask struct {
	Host    string `json:"host"`
	Latency int64  `json:"latency_ms"`
}

func (t *PingTask) Execute(ctx context.Context) (*PingTask, error) {
	select {
	case <-ctx.Done():
		return t, ctx.Err()
	case <-time.After(time.Duration(rand.Intn(50)) * time.Millisecond):
	}
	t.Latency = rand.Int63n(100)
	return t, nil
}

func main() {
	ctx := context.Background()

	hosts := []string{"a.example", "b.example", "c.example", "d.example"}
	tasks := make(chan gojob.Task[*PingTask])
	go func() {
		defer close(tasks)
		for _, h := range hosts {
			tasks <- &PingTask{Host: h}
		}
	}()

	results := gojob.Execute(ctx, tasks, gojob.WithWorkers(4))
	if err := gojob.WriteJSONL(ctx, os.Stdout, results); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
