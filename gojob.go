package gojob

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Task is an interface that defines a task
type Task interface {
	// Do starts the task
	Do() error
}

type BasicTask struct {
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
	NumTries   int    `json:"num_tries"`
	Task       Task   `json:"task"`
	Error      string `json:"error"`
}

func NewBasicTask(task Task) *BasicTask {
	return &BasicTask{
		Task: task,
	}
}

// Scheduler is a task scheduler
type Scheduler struct {
	NumWorkers               int
	OutputFilePath           string
	MaxRetries               int
	MaxRuntimePerTaskSeconds int
	TaskChan                 chan Task
	LogChan                  chan string
	taskWg                   *sync.WaitGroup
	logWg                    *sync.WaitGroup
}

// NewScheduler creates a new scheduler
func NewScheduler(numWorkers int, maxRetries int, maxRuntimePerTaskSeconds int, outputFilePath string) *Scheduler {
	scheduler := &Scheduler{
		NumWorkers:               numWorkers,
		OutputFilePath:           outputFilePath,
		MaxRetries:               maxRetries,
		MaxRuntimePerTaskSeconds: maxRuntimePerTaskSeconds,
		TaskChan:                 make(chan Task),
		LogChan:                  make(chan string),
		taskWg:                   &sync.WaitGroup{},
		logWg:                    &sync.WaitGroup{},
	}
	scheduler.Start()
	return scheduler
}

// Submit submits a task to the scheduler
func (s *Scheduler) Submit(task Task) {
	s.taskWg.Add(1)
	s.TaskChan <- task
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	for i := 0; i < s.NumWorkers; i++ {
		go s.Worker()
	}
	go s.Writer()
}

// Wait waits for all tasks to finish
func (s *Scheduler) Wait() {
	s.taskWg.Wait()
	close(s.TaskChan)
	s.logWg.Wait()
	close(s.LogChan)
}

// Worker is a worker
func (s *Scheduler) Worker() {
	for task := range s.TaskChan {
		// Start task
		bt := NewBasicTask(task)
		for i := 0; i < s.MaxRetries; i++ {
			err := func() error {
				bt.StartedAt = time.Now().UnixMicro()
				defer func() {
					bt.NumTries++
					bt.FinishedAt = time.Now().UnixMicro()
				}()
				return RunWithTimeout(task.Do, time.Duration(s.MaxRuntimePerTaskSeconds)*time.Second)
			}()
			if err != nil {
				bt.Error = err.Error()
			} else {
				bt.Error = ""
				break
			}
		}
		// Serialize task
		data, err := json.Marshal(bt)
		if err != nil {
			slog.Error("error occured while serializing task", slog.String("error", err.Error()))
		} else {
			s.logWg.Add(1)
			s.LogChan <- string(data)
		}
		// Notify task is done
		s.taskWg.Done()
	}
}

// Writer writes logs to file
func (s *Scheduler) Writer() {
	var fd *os.File
	var err error
	if s.OutputFilePath == "-" {
		fd = os.Stdout
	} else {
		// Create folder if not exists
		dir := filepath.Dir(s.OutputFilePath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			err = os.MkdirAll(dir, 0755)
			if err != nil {
				slog.Error("error occured while creating folder", slog.String("path", dir), slog.String("error", err.Error()))
				return
			}
		}
		// Open file
		fd, err = os.OpenFile(s.OutputFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			slog.Error("error occured while opening file", slog.String("path", s.OutputFilePath), slog.String("error", err.Error()))
			return
		}
		defer fd.Close()
	}
	for result := range s.LogChan {
		fd.WriteString(result + "\n")
		s.logWg.Done()
	}
}

func RunWithTimeout(f func() error, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		done <- f()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}
