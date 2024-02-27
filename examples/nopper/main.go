package main

import (
	"math/rand"
	"time"

	"github.com/WangYihang/gojob"
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
	scheduler := gojob.NewScheduler().
		SetNumWorkers(8).
		SetMaxRetries(4).
		SetMaxRuntimePerTaskSeconds(16).
		SetNumShards(4).
		SetShard(0).
		SetOutputFilePath("test.txt").
		Start()
	for i := 0; i < 257; i++ {
		scheduler.Submit(New())
	}
	scheduler.Wait()
}
