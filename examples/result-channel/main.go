package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
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

func (t *MyTask) Do(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.Url, nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	t.StatusCode = response.StatusCode
	return nil
}

func main() {
	var numTotalTasks int64 = 256
	scheduler := gojob.New(
		gojob.WithNumWorkers(8),
		gojob.WithMaxRetries(4),
		gojob.WithMaxRuntimePerTaskSeconds(16),
		gojob.WithNumShards(4),
		gojob.WithShard(0),
		gojob.WithTotalTasks(numTotalTasks),
		gojob.WithStatusFilePath("status.json"),
		gojob.WithResultFilePath("result.json"),
		gojob.WithMetadataFilePath("metadata.json"),
	)
	// Register the result consumer before the scheduler starts so that no
	// results are missed.
	resultChan := scheduler.ResultChan()
	go func() {
		for result := range resultChan {
			data, err := json.Marshal(result)
			if err != nil {
				slog.Error("failed to marshal result", slog.String("error", err.Error()))
				continue
			}
			fmt.Println(string(data))
		}
	}()
	for i := range numTotalTasks {
		scheduler.Submit(New(fmt.Sprintf("https://httpbin.org/task/%d", i)))
	}
	scheduler.Wait()
}
