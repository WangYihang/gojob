package pipekit

import (
	"bufio"
	"bytes"
	"log/slog"
	"os"
	"sync"
)

// Fanin takes a slice of channels and returns a single channel that
func Fanin[T interface{}](cs []chan T) chan T {
	var wg sync.WaitGroup
	out := make(chan T)
	output := func(c chan T) {
		for n := range c {
			out <- n
		}
		wg.Done()
	}
	wg.Add(len(cs))
	for _, c := range cs {
		go output(c)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// Fanout takes a channel and returns a slice of channels
// the item in the input channel will be distributed to the output channels
func Fanout[T interface{}](in chan *T, n int) []chan *T {
	cs := make([]chan *T, n)
	for i := 0; i < n; i++ {
		cs[i] = make(chan *T)
		go func(c chan *T) {
			for n := range in {
				c <- n
			}
			close(c)
		}(cs[i])
	}
	return cs
}

// Head takes a channel and returns a channel with the first n items
func Head[T interface{}](in chan T, max int) chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		i := 0
		for line := range in {
			if i >= max {
				break
			}
			out <- line
			i++
		}
	}()
	return out
}

// Tail takes a channel and returns a channel with the last n items
func Tail[T interface{}](in chan T, max int) chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		var lines []T
		for line := range in {
			lines = append(lines, line)
			if len(lines) > max {
				lines = lines[1:]
			}
		}
		for _, line := range lines {
			out <- line
		}
	}()
	return out
}

// Cat takes a file path and returns a channel with the lines of the file
// each line is trimmed of whitespace
func Cat(path string) chan []byte {
	out := make(chan []byte)
	go func() {
		defer close(out)
		fd, err := os.Open(path)
		if err != nil {
			slog.Error("error occured while opening file", slog.String("path", path), slog.String("error", err.Error()))
			return
		}
		defer fd.Close()
		scanner := bufio.NewScanner(fd)
		for scanner.Scan() {
			out <- bytes.TrimSpace(scanner.Bytes())
		}
	}()
	return out
}

// Filter takes a channel and returns a channel with the items that pass the filter
func Filter[T interface{}](in chan T, f func(T) bool) chan T {
	out := make(chan T)
	go func() {
		defer close(out)
		for line := range in {
			if f(line) {
				out <- line
			}
		}
	}()
	return out
}

// Map takes a channel and returns a channel with the items that pass the filter
func Map[T interface{}, U interface{}](in chan T, f func(T) U) chan U {
	out := make(chan U)
	go func() {
		defer close(out)
		for line := range in {
			out <- f(line)
		}
	}()
	return out
}

// Reduce takes a channel and returns a channel with the items that pass the filter
func Reduce[T interface{}](in chan T, f func(T, T) T) T {
	var result T
	for line := range in {
		result = f(result, line)
	}
	return result
}

// Task is an interface that defines a task
type Task interface {
	// Parse unserializes a task from a byte array, returns an error if the task is invalid
	// For example, a task can be unserialized from a line of a file
	Parse(data []byte) (err error)
	// Start starts the task
	Start()
	// Bytes serializes a task to a byte array, returns an error if the task is invalid
	// For example, a task can be serialized to a line of a file
	// You can store the result of a task in the task itself, when the task is serialized, the bytes of the result will be written to the log file
	Bytes() ([]byte, error)
}

// Scheduler is a task scheduler
type Scheduler struct {
	NumWorkers     int
	OutputFilePath string
	TaskWaitGroup  sync.WaitGroup
	LogWaitGroup   sync.WaitGroup
	TaskChan       chan Task
	StopChan       chan struct{}
	LogChan        chan string
}

// NewScheduler creates a new scheduler
func NewScheduler(numWorkers int, outputFilePath string) *Scheduler {
	s := &Scheduler{
		NumWorkers:    numWorkers,
		TaskWaitGroup: sync.WaitGroup{},
		LogWaitGroup:  sync.WaitGroup{},
		TaskChan:      make(chan Task, numWorkers),
		StopChan:      make(chan struct{}),
		LogChan:       make(chan string),
	}
	go s.Logger(outputFilePath)
	return s
}

// Add adds a task to the scheduler
func (s *Scheduler) Add(task Task) {
	s.TaskWaitGroup.Add(1)
	s.TaskChan <- task
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	for i := 0; i < s.NumWorkers; i++ {
		go s.Worker()
	}
	s.TaskWaitGroup.Wait()
	close(s.TaskChan)
	s.LogWaitGroup.Wait()
	close(s.LogChan)
}

// Worker is a worker that executes tasks
func (s *Scheduler) Worker() {
	for {
		select {
		case task, ok := <-s.TaskChan:
			if !ok {
				return
			}
			task.Start()
			data, _ := task.Bytes()
			s.TaskWaitGroup.Done()
			s.LogWaitGroup.Add(1)
			s.LogChan <- string(data)
		case <-s.StopChan:
			return
		}
	}
}

func (s *Scheduler) Logger(outputFilePath string) {
	fd, err := os.OpenFile(outputFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		slog.Error("error occured while opening file", slog.String("path", outputFilePath), slog.String("error", err.Error()))
		return
	}
	defer fd.Close()
	for line := range s.LogChan {
		fd.WriteString(line + "\n")
		s.LogWaitGroup.Done()
	}
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	close(s.StopChan)
}
