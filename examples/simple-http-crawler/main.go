package main

import (
	"net/http"

	"github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/pkg/util"
)

type MyTask struct {
	Url        string `json:"url"`
	StatusCode int    `json:"status_code"`
}

func New(url string) *MyTask {
	return &MyTask{
		Url: url,
	}
}

func (t *MyTask) Do() error {
	response, err := http.Get(t.Url)
	if err != nil {
		return err
	}
	t.StatusCode = response.StatusCode
	defer response.Body.Close()
	return nil
}

func main() {
	scheduler := gojob.NewScheduler().
		SetNumWorkers(8).
		SetMaxRetries(4).
		SetMaxRuntimePerTaskSeconds(16).
		SetNumShards(4).
		SetShard(0).
		Start()
	for line := range util.Cat("input.txt") {
		scheduler.Submit(New(line))
	}
	scheduler.Wait()
}
