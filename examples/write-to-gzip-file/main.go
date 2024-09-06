package main

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/WangYihang/gojob"
)

type MyTask struct {
	Line string
}

func New(line string) *MyTask {
	return &MyTask{
		Line: line,
	}
}

func (t *MyTask) Do() error {
	time.Sleep(time.Duration(rand.Intn(1000)) * time.Millisecond)
	fmt.Println(t.Line)
	return nil
}

func main() {
	scheduler := gojob.New(
		gojob.WithNumWorkers(1),
		gojob.WithMaxRetries(4),
		gojob.WithMaxRuntimePerTaskSeconds(16),
		gojob.WithResultFilePath("result.txt.gz"),
		gojob.WithStatusFilePath("status.json"),
		gojob.WithMetadataFilePath("metadata.json"),
	).
		Start()
	for line := range 16 {
		scheduler.Submit(New(fmt.Sprintf("line-%d", line)))
	}
	scheduler.Wait()
}
