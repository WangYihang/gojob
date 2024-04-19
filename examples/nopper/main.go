package main

import (
	"math/rand"
	"time"

	"github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/pkg/utils"
)

type MyTask struct{}

func New() *MyTask {
	return &MyTask{}
}

func (t *MyTask) Do() error {
	time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
	return nil
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
	).
		Start()
	for range utils.Cat(inputFilePath) {
		scheduler.Submit(New())
	}
	scheduler.Wait()
}
