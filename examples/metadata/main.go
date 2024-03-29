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
	time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
	return nil
}

func main() {
	inputFilePath := "data/input.txt"
	total := utils.Count(utils.Cat(inputFilePath))
	scheduler := gojob.NewScheduler().
		SetNumWorkers(8).
		SetMaxRetries(4).
		SetMaxRuntimePerTaskSeconds(16).
		SetNumShards(4).
		SetShard(0).
		SetOutputFilePath("data/output.txt").
		SetStatusFilePath("data/output.status").
		SetTotalTasks(total).
		SetMetadata("a", "b").
		SetMetadata("c", "d").
		Start()
	for range utils.Cat(inputFilePath) {
		scheduler.Submit(New())
	}
	scheduler.Wait()
}
