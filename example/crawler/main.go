package main

import (
	"encoding/json"
	"log/slog"
	"os"
	"strings"

	"github.com/WangYihang/go-pipekit"
	"github.com/WangYihang/go-pipekit/example/crawler/pkg/model"
	"github.com/jessevdk/go-flags"
)

type Options struct {
	InputFilePath  string `short:"i" long:"input" description:"input file path" required:"true"`
	OutputFilePath string `short:"o" long:"output" description:"output file path" required:"true"`
	NumWorkers     int    `short:"n" long:"num-workers" description:"number of workers" default:"32"`
}

var Opts Options

func init() {
	_, err := flags.Parse(&Opts)
	if err != nil {
		os.Exit(1)
	}
}

func loader() chan *model.Task {
	return pipekit.Filter(
		pipekit.Map(
			pipekit.Filter(
				pipekit.Map(
					pipekit.Cat(Opts.InputFilePath),
					// Convert []byte to string
					func(line []byte) string {
						return string(line)
					},
				),
				// Filter out comment lines
				func(line string) bool {
					return !strings.HasPrefix(line, "#")
				},
			),
			// Convert string to Task
			func(line string) *model.Task {
				u, err := model.NewTask(line)
				if err != nil {
					slog.Error("error occured while creating task", slog.String("error", err.Error()))
					return nil
				}
				return u
			},
		),
		// Filter out nil tasks
		func(t *model.Task) bool {
			return t != nil
		},
	)
}

func worker(tasks chan *model.Task) chan *model.Result {
	out := make(chan *model.Result)
	go func() {
		defer close(out)
		for task := range tasks {
			out <- task.Run()
		}
	}()
	return out
}

func scheduler(tasks chan *model.Task, n int) []chan *model.Result {
	results := []chan *model.Result{}
	for i := 0; i < n; i++ {
		results = append(results, worker(tasks))
	}
	return results
}

func writer(results chan *model.Result) {
	fd, err := os.OpenFile(Opts.OutputFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.Error("error occured while opening file", slog.String("path", Opts.OutputFilePath), slog.String("error", err.Error()))
		return
	}
	encoder := json.NewEncoder(fd)
	for result := range results {
		encoder.Encode(result)
	}
}

func main() {
	writer(pipekit.Fanin(scheduler(loader(), Opts.NumWorkers)))
}
