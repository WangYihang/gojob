package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"time"

	pipekit "github.com/WangYihang/GoJob"
)

type MyTask struct {
	Url        string `json:"url"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
	StatusCode int    `json:"status_code"`
	Error      string `json:"error"`
}

func NewTask(line []byte) *MyTask {
	t := &MyTask{}
	t.Unserialize(line)
	return t
}

func (t *MyTask) Unserialize(line []byte) (err error) {
	t.Url = string(bytes.TrimSpace(line))
	return
}

func (t *MyTask) Start() {
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

func (t *MyTask) Serialize() ([]byte, error) {
	return json.Marshal(t)
}

func main() {
	scheduler := pipekit.NewScheduler(16, "output.txt")
	for line := range pipekit.Cat("input.txt") {
		scheduler.Add(NewTask(line))
	}
	scheduler.Start()
}
