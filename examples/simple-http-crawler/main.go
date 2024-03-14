package main

import (
	"net/http"

	"github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/pkg/utils"
)

type MyTask struct {
	Url        string `json:"url"`
	StatusCode int    `json:"status_code"`
}

func NewTask(url string) *MyTask {
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
	inputFilePath := "data/input.txt"
	total := utils.Count(utils.Cat(inputFilePath))
	scheduler := gojob.NewScheduler().
		SetNumWorkers(8).
		SetMaxRetries(4).
		SetMaxRuntimePerTaskSeconds(16).
		SetNumShards(4).
		SetShard(0).
		SetTotalTasks(total).
		Start()
	for line := range utils.Cat("data/input.txt") {
		scheduler.Submit(NewTask(line))
	}
	scheduler.Wait()
}
