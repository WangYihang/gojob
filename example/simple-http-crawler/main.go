package main

import (
	"net/http"

	"github.com/WangYihang/gojob"
)

type MyTask struct {
	Url        string `json:"url"`
	StatusCode int    `json:"status_code"`
}

func New(url string) *MyTask {
	return &MyTask{
		Url:        url,
		StatusCode: 0,
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
	scheduler := gojob.NewScheduler(1, 4, 8, "output.txt")
	for line := range gojob.Cat("input.txt") {
		scheduler.Submit(New(line))
	}
	scheduler.Wait()
}
