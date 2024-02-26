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
	scheduler := gojob.NewScheduler(8, 4, 16, "output.txt")
	scheduler.Start()
	for i := 0; i < 256; i++ {
		scheduler.Submit(New(i, rand.Intn(10)))
	}
	scheduler.Wait()
}
