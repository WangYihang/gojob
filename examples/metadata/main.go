package main

import (
	"context"
	"math/rand"
	"time"

	"github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/pkg/runner"
	"github.com/WangYihang/gojob/pkg/utils"
)

type MyTask struct{}

func New() *MyTask {
	return &MyTask{}
}

func (t *MyTask) Do(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Duration(rand.Intn(100)) * time.Millisecond):
		return nil
	}
}

func main() {
	inputFilePath := "data/input.txt"
	total := utils.Count(utils.Cat(inputFilePath))
	scheduler := gojob.New(
		gojob.WithNumWorkers(8),
		gojob.WithMaxRetries(4),
		gojob.WithMaxRuntimePerTaskSeconds(16),
		gojob.WithNumShards(4),
		gojob.WithShard(0),
		gojob.WithResultFilePath("data/output.txt"),
		gojob.WithStatusFilePath("data/output.status"),
		gojob.WithTotalTasks(total),
		gojob.WithMetadata("a", "b"),
		gojob.WithMetadata("c", "d"),
		gojob.WithMetadata("runner", runner.Get()),
	).
		Start()
	for range utils.Cat(inputFilePath) {
		scheduler.Submit(New())
	}
	scheduler.Wait()
}
