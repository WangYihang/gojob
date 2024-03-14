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
	scheduler := gojob.NewScheduler().
		SetNumWorkers(8).
		SetMaxRetries(4).
		SetMaxRuntimePerTaskSeconds(16).
		SetNumShards(4).
		SetShard(0).
		SetOutputFilePath("output.txt").
		SetTotalTasks(int64(total)).
		Start()
	for i := 0; i < total; i++ {
		scheduler.Submit(New(i, rand.Intn(10)))
	}
	scheduler.Wait()
}
