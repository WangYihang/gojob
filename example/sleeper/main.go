package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/WangYihang/gojob"
	grmon "github.com/bcicen/grmon/agent"
)

var MAX_TRIES = 4

func init() {
	grmon.Start()
}

type MyTask struct {
	Index        int    `json:"index"`
	SleepSeconds int    `json:"sleep_seconds"`
	StartedAt    int64  `json:"started_at"`
	FinishedAt   int64  `json:"finished_at"`
	Error        string `json:"error"`
	NumTries     int    `json:"num_tries"`
}

func New(index int, sleepSeconds int) *MyTask {
	return &MyTask{
		Index:        index,
		SleepSeconds: sleepSeconds,
	}
}

func (t *MyTask) Do(ctx context.Context) error {
	t.NumTries++
	t.StartedAt = time.Now().UnixMilli()
	defer func() {
		t.FinishedAt = time.Now().UnixMilli()
	}()
	slog.Info("start processing", slog.Int("sleep_seconds", t.SleepSeconds), slog.Int("num_tries", t.NumTries))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			time.Sleep(8 * time.Second)
		}
	}
}

func (t *MyTask) Bytes() ([]byte, error) {
	return json.Marshal(t)
}

func (t *MyTask) NeedRetry() bool {
	return t.NumTries < MAX_TRIES
}

func main() {
	scheduler := gojob.NewScheduler(8, 1, "output.txt")
	scheduler.Start()
	for i := 0; i < 256; i++ {
		scheduler.Submit(New(i, 8))
	}
	scheduler.Wait()
}
