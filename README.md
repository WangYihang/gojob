# go(od)job

gojob is a simple job scheduler.

## Install

```
go get github.com/WangYihang/gojob
```

## Usage

Create a job scheduler with a worker pool of size 32. To do this, you need to implement the `Task` interface.

```go
// Task is an interface that defines a task
type Task interface {
	// Do starts the task
	Do() error
	// Bytes serializes a task to a byte array, returns an error if the task is invalid
	// For example, a task can be serialized to a line of a file
	// You can store the result of a task in the task itself, when the task is serialized, the bytes of the result will be written to the log file
	Bytes() ([]byte, error)
	// NeedRetry returns true if the task needs to be retried
	NeedRetry() bool
}
```

The whole [code](./example/simple-http-crawler/) looks like this.

```go
package main

import (
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

func (t *MyTask) Do() error {
	t.NumTries++
	t.StartedAt = time.Now().UnixMilli()
	defer func() {
		t.FinishedAt = time.Now().UnixMilli()
	}()
	response, err := http.Get(t.Url)
	if err != nil {
		t.Error = err.Error()
		return err
	}
	t.StatusCode = response.StatusCode
	defer response.Body.Close()
	return nil
}

func (t *MyTask) Bytes() ([]byte, error) {
	return json.Marshal(t)
}

func (t *MyTask) NeedRetry() bool {
	return t.NumTries < MAX_TRIES
}

func main() {
	scheduler := gojob.NewScheduler(1, "output.txt")
	scheduler.Start()
	for line := range gojob.Cat("input.txt") {
		scheduler.Submit(New(line))
	}
	scheduler.Wait()
}
```

## Use Case

### http-crawler

Let's say you have a bunch of URLs that you want to crawl and save the HTTP response to a file. You can use gojob to do that.
Check [it](./example/complex-http-crawler/) out for details.

Try it out using the following command.

```bash
$ go run github.com/WangYihang/gojob/example/complex-http-crawler@latest --help                       
Usage:
  main [OPTIONS]

Application Options:
  -i, --input=       input file path
  -o, --output=      output file path
  -n, --num-workers= number of workers (default: 32)

Help Options:
  -h, --help         Show this help message

exit status 1
```

```bash
$ cat urls.txt
https://www.google.com/
https://www.facebook.com/
https://www.youtube.com/
```

```
$ go run github.com/WangYihang/gojob/example/complex-http-crawler@latest -i input.txt -o output.txt -n 4
```

```json
$ tail -n 1 output.txt
{
    "url": "https://www.youtube.com/",
    "http": {
        "request": {
            "method": "HEAD",
            "url": "https://www.youtube.com/",
            "host": "www.youtube.com",
            "proto": "HTTP/1.1"
        },
        "response": {
            "status": "200 OK",
            "status_code": 200,
            "proto": "HTTP/2.0",
            "proto_major": 2,
            "proto_minor": 0,
            "header": {
                "Server": [
                    "ESF"
                ]
            },
            // details omitted for simplicity
            "body": "",
            "content_length": 783869
        }
    },
    "error": ""
}
```