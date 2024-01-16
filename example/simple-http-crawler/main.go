package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/WangYihang/gojob"
)

type MyTask struct {
	Url        string `json:"url"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error"`
}

func New(url string) *MyTask {
	return &MyTask{
		Url: url,
	}
}

func (t *MyTask) Do() {
	t.StartedAt = time.Now().UnixMilli()
	defer func() {
		t.FinishedAt = time.Now().UnixMilli()
	}()
	response, err := http.Get(t.Url)
	if err != nil {
		t.Error = err.Error()
		return
	}
	t.StatusCode = response.StatusCode
	defer response.Body.Close()
}

func (t *MyTask) Bytes() ([]byte, error) {
	return json.Marshal(t)
}

func main() {
	scheduler := gojob.NewScheduler(16, "output.txt")
	go func() {
		for line := range gojob.Cat("input.txt") {
			scheduler.Add(New(string(line)))
		}
	}()
	scheduler.Start()
}
