package main

import (
	"context"
	"math/rand"
	"time"

	"github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/pkg/utils"
)

type MyTask struct {
	Line string
}

func New(line string) *MyTask {
	return &MyTask{
		Line: line,
	}
}

func (t *MyTask) Do(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(rand.Intn(1000)) * time.Millisecond):
		return nil
	}
}

func main() {
	scheduler := gojob.New(
		gojob.WithNumWorkers(8),
		gojob.WithMaxRetries(4),
		gojob.WithMaxRuntimePerTaskSeconds(16),
		gojob.WithResultFilePath("-"),
		gojob.WithStatusFilePath("status.json"),
	).
		Start()
	for line := range utils.Cat("data.txt.gz") {
		scheduler.Submit(New(line))
	}
	scheduler.Wait()
}
