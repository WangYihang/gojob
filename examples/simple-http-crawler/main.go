package main

import (
	"fmt"
	"net/http"

	"github.com/WangYihang/gojob"
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
	var numTotalTasks int64 = 256
	scheduler := gojob.NewScheduler().
		SetNumWorkers(8).
		SetMaxRetries(4).
		SetOutputFilePath("output.json").
		SetMaxRuntimePerTaskSeconds(16).
		SetNumShards(4).
		SetShard(0).
		SetTotalTasks(numTotalTasks).
		Start()
	for i := range numTotalTasks {
		scheduler.Submit(New(fmt.Sprintf("https://httpbin.org/task/%d", i)))
	}
	scheduler.Wait()
}
