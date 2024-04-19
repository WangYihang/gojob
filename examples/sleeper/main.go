package main

import (
	"math/rand"
	"time"

	"github.com/WangYihang/gojob"
)

type MyTask struct {
	Index        int `json:"index"`
	SleepSeconds int `json:"sleep_seconds"`
}

func New(index int, sleepSeconds int) *MyTask {
	return &MyTask{
		Index:        index,
		SleepSeconds: sleepSeconds,
	}
}

func (t *MyTask) Do() error {
	time.Sleep(time.Duration(t.SleepSeconds) * time.Second)
	return nil
}

func main() {
	total := 256
	scheduler := gojob.New(
		gojob.WithNumWorkers(8),
		gojob.WithMaxRetries(4),
		gojob.WithMaxRuntimePerTaskSeconds(16),
		gojob.WithNumShards(4),
		gojob.WithShard(0),
		gojob.WithTotalTasks(int64(total)),
		gojob.WithStatusFilePath("-"),
		gojob.WithResultFilePath("-"),
		gojob.WithMetadataFilePath("-"),
	).Start()

	for i := 0; i < total; i++ {
		scheduler.Submit(New(i, rand.Intn(10)))
	}
	scheduler.Wait()
}
