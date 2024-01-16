# go(od)job

gojob is a simple job scheduler.

## Install

```
go get github.com/WangYihang/gojob
```

## Usage

Create a job scheduler with a worker pool of size 32. To do this, you need to implement the `Task` interface.

```go
type Task interface {
	Do()
	Bytes() ([]byte, error)
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
```

## Use Case

### http-crawler

Let's say you have a bunch of URLs that you want to crawl and save the HTTP response to a file. You can use gojob to do that.
Check [it](./example/complex-http-crawler/) out for details.

```
$ cat urls.txt
https://www.google.com/
https://www.facebook.com/
https://www.youtube.com/
```

```bash
$ go run example/complex-http-crawler/main.go --help                         
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

```
$ go run example/complex-http-crawler/main.go -i input.txt -o output.txt -n 4
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