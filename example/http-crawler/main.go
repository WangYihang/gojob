package main

import (
	"os"

	pipekit "github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/example/http-crawler/pkg/model"
	"github.com/jessevdk/go-flags"
)

type Options struct {
	InputFilePath  string `short:"i" long:"input" description:"input file path" required:"true"`
	OutputFilePath string `short:"o" long:"output" description:"output file path" required:"true"`
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
	scheduler := pipekit.NewScheduler(32, "output.txt")
	for line := range pipekit.Cat("input.txt") {
		scheduler.Add(model.NewTask(line))
	}
	scheduler.Start()
}
