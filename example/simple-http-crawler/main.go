package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/WangYihang/gojob"
)

var MAX_TRIES = 4

type MyTask struct {
	Url        string `json:"url"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error"`
	NumTries   int    `json:"num_tries"`
}

func New(url string) *MyTask {
	return &MyTask{
		Url: url,
	}
}

func (t *MyTask) Do(ctx context.Context) error {
	t.NumTries++
	t.StartedAt = time.Now().UnixMilli()
	defer func() {
		t.FinishedAt = time.Now().UnixMilli()
	}()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			response, err := http.Get(t.Url)
			if err != nil {
				t.Error = err.Error()
				return err
			}
			t.StatusCode = response.StatusCode
			defer response.Body.Close()
			return nil
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
	scheduler := gojob.NewScheduler(1, 8, "output.txt")
	scheduler.Start()
	for line := range gojob.Cat("input.txt") {
		scheduler.Submit(New(line))
	}
	scheduler.Wait()
}
