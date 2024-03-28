package main

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/WangYihang/gojob"
)

type MyTask struct {
	Index            int     `json:"index"`
	SleepSeconds     int     `json:"sleep_seconds"`
	ErrorProbability float64 `json:"error_probability"`
}

func New(index int, sleepSeconds int) *MyTask {
	return &MyTask{
		Index:            index,
		SleepSeconds:     sleepSeconds,
		ErrorProbability: 0.32,
	}
}

func (t *MyTask) Do() error {
	time.Sleep(time.Duration(t.SleepSeconds) * time.Second)
	if rand.Float64() < t.ErrorProbability {
		fmt.Println(">>>", "error")
		return errors.New("an error occurred")
	}
	return nil
}

func main() {
	total := 256
	scheduler := gojob.NewScheduler().
		SetNumWorkers(8).
		SetMaxRetries(1).
		SetMaxRuntimePerTaskSeconds(16).
		SetShard(0).
		SetNumShards(1).
		SetOutputFilePath("output.txt").
		SetTotalTasks(int64(total)).
		Start()
	for i := 0; i < total; i++ {
		scheduler.Submit(New(i, rand.Intn(10)))
	}
	scheduler.Wait()
}
