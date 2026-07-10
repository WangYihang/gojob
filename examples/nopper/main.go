package main

import (
	"context"
	"math/rand"
	"time"

	"github.com/WangYihang/gojob"
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
	case <-time.After(time.Duration(rand.Intn(1000)) * time.Millisecond):
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
		gojob.WithResultFilePath("data/output.json"),
		gojob.WithStatusFilePath("data/status.json"),
		gojob.WithMetadataFilePath("data/metadata.json"),
		gojob.WithTotalTasks(total),
	).
		Start()
	for range utils.Cat(inputFilePath) {
		scheduler.Submit(New())
	}
	scheduler.Wait()
}
