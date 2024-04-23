package main

import (
	"errors"
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
		return errors.New("an error occurred")
	}
	return nil
}

func main() {
	total := 256
	scheduler := gojob.New(
		gojob.WithNumWorkers(8),
		gojob.WithMaxRetries(1),
		gojob.WithMaxRuntimePerTaskSeconds(16),
		gojob.WithShard(0),
		gojob.WithNumShards(1),
		gojob.WithTotalTasks(int64(total)),
		gojob.WithStatusFilePath("status.json"),
		gojob.WithResultFilePath("result.json"),
		gojob.WithMetadataFilePath("metadata.json"),
	).Start()
	for i := 0; i < total; i++ {
		scheduler.Submit(New(i, rand.Intn(10)))
	}
	scheduler.Wait()
}
