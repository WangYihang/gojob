package main

import (
	"context"
	"fmt"

	"github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/pkg/utils"
)

type MyPrinterTask struct {
	line string
}

func New(line string) *MyPrinterTask {
	return &MyPrinterTask{
		line: line,
	}
}

func (t *MyPrinterTask) Do(ctx context.Context) error {
	fmt.Println(t.line)
	return nil
}

func main() {
	scheduler := gojob.New(
		gojob.WithNumWorkers(8),
		gojob.WithMaxRetries(4),
		gojob.WithMaxRuntimePerTaskSeconds(16),
		gojob.WithResultFilePath("-"),
		gojob.WithStatusFilePath("status.json"),
	).
		Start()
	for line := range utils.Cat("-") {
		scheduler.Submit(New(line))
	}
	scheduler.Wait()
}
