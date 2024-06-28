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
	scheduler := gojob.New(
		gojob.WithNumWorkers(8),
		gojob.WithMaxRetries(4),
		gojob.WithMaxRuntimePerTaskSeconds(16),
		gojob.WithNumShards(4),
		gojob.WithShard(0),
		gojob.WithResultFilePath("-"),
		gojob.WithStatusFilePath("status.json"),
	).
		Start()
	for range utils.Cat("-") {
		scheduler.Submit(New())
	}
	scheduler.Wait()
}
