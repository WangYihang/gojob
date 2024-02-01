package main

import (
	"os"

	"github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/example/complex-http-crawler/pkg/model"
	"github.com/jessevdk/go-flags"
)

type Options struct {
	InputFilePath  string `short:"i" long:"input" description:"input file path" required:"true"`
	OutputFilePath string `short:"o" long:"output" description:"output file path" required:"true"`
	TimeoutPerTask int    `short:"t" long:"timeout" description:"timeout per task in seconds" default:"8"`
	NumWorkers     int    `short:"n" long:"num-workers" description:"number of workers" default:"32"`
}

var opts Options

func init() {
	_, err := flags.Parse(&opts)
	if err != nil {
		os.Exit(1)
	}
}

func main() {
	scheduler := gojob.NewScheduler(opts.NumWorkers, opts.TimeoutPerTask, opts.OutputFilePath)
	for line := range gojob.Cat(opts.InputFilePath) {
		scheduler.Submit(model.New(string(line)))
	}
	scheduler.Start()
	scheduler.Wait()
}
