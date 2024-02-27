package gojob

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

// Task is an interface that defines a task
type Task interface {
	// Do starts the task
	Do() error
}

type BasicTask struct {
	Index      int64  `json:"index"`
	StartedAt  int64  `json:"started_at"`
	FinishedAt int64  `json:"finished_at"`
	NumTries   int    `json:"num_tries"`
	Task       Task   `json:"task"`
	Error      string `json:"error"`
}

func NewBasicTask(index int64, task Task) *BasicTask {
	return &BasicTask{
		Index:      index,
		StartedAt:  0,
		FinishedAt: 0,
		NumTries:   0,
		Task:       task,
		Error:      "",
	}
}

// Scheduler is a task scheduler
type Scheduler struct {
	NumWorkers               int
	OutputFilePath           string
	MaxRetries               int
	MaxRuntimePerTaskSeconds int
	NumShards                int64
	Shard                    int64
	IsStarted                bool
	NumTasks                 atomic.Int64
	TaskChan                 chan *BasicTask
	LogChan                  chan string
	taskWg                   *sync.WaitGroup
	logWg                    *sync.WaitGroup
}

// NewScheduler creates a new scheduler
func NewScheduler() *Scheduler {
	scheduler := &Scheduler{
		NumWorkers:               1,
		OutputFilePath:           "-",
		MaxRetries:               4,
		MaxRuntimePerTaskSeconds: 16,
		NumShards:                3,
		Shard:                    1,
		IsStarted:                false,
		NumTasks:                 atomic.Int64{},
		TaskChan:                 make(chan *BasicTask),
		LogChan:                  make(chan string),
		taskWg:                   &sync.WaitGroup{},
		logWg:                    &sync.WaitGroup{},
	}
	return scheduler
}

// SetNumShards sets the number of shards, default is 1 which means no sharding
func (s *Scheduler) SetNumShards(numShards int64) *Scheduler {
	if numShards <= 0 {
		panic("numShards must be greater than 0")
	}
	s.NumShards = numShards
	return s
}

// SetShard sets the shard (from 0 to NumShards-1)
func (s *Scheduler) SetShard(shard int64) *Scheduler {
	if shard < 0 || shard >= s.NumShards {
		panic("shard must be in [0, NumShards)")
	}
	s.Shard = shard
	return s
}

// SetNumWorkers sets the number of workers
func (s *Scheduler) SetNumWorkers(numWorkers int) *Scheduler {
	if numWorkers <= 0 {
		panic("numWorkers must be greater than 0")
	}
	s.NumWorkers = numWorkers
	return s
}

// SetOutputFilePath sets the output file path
func (s *Scheduler) SetOutputFilePath(outputFilePath string) *Scheduler {
	s.OutputFilePath = outputFilePath
	return s
}

// SetMaxRetries sets the max retries
func (s *Scheduler) SetMaxRetries(maxRetries int) *Scheduler {
	if maxRetries <= 0 {
		panic("maxRetries must be greater than 0")
	}
	s.MaxRetries = maxRetries
	return s
}

// SetMaxRuntimePerTaskSeconds sets the max runtime per task seconds
func (s *Scheduler) SetMaxRuntimePerTaskSeconds(maxRuntimePerTaskSeconds int) *Scheduler {
	if maxRuntimePerTaskSeconds <= 0 {
		panic("maxRuntimePerTaskSeconds must be greater than 0")
	}
	s.MaxRuntimePerTaskSeconds = maxRuntimePerTaskSeconds
	return s
}

// Submit submits a task to the scheduler
func (s *Scheduler) Submit(task Task) {
	if !s.IsStarted {
		s.Start()
		s.IsStarted = true
	}
	index := s.NumTasks.Load()
	if (index % s.NumShards) == s.Shard {
		s.taskWg.Add(1)
		s.TaskChan <- NewBasicTask(index, task)
	}
	s.NumTasks.Add(1)
}

// Start starts the scheduler
func (s *Scheduler) Start() *Scheduler {
	if s.IsStarted {
		return s
	}
	for i := 0; i < s.NumWorkers; i++ {
		go s.Worker()
	}
	go s.Writer()
	return s
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
		for i := 0; i < s.MaxRetries; i++ {
			err := func() error {
				task.StartedAt = time.Now().UnixMicro()
				defer func() {
					task.NumTries++
					task.FinishedAt = time.Now().UnixMicro()
				}()
				return RunWithTimeout(task.Task.Do, time.Duration(s.MaxRuntimePerTaskSeconds)*time.Second)
			}()
			if err != nil {
				task.Error = err.Error()
			} else {
				task.Error = ""
				break
			}
		}
		// Serialize task
		data, err := json.Marshal(task)
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
	var fd io.Writer
	var err error

	switch s.OutputFilePath {
	case "-":
		fd = os.Stdout
	case "":
		fd = io.Discard
	default:
		// Create folder if not exists
		dir := filepath.Dir(s.OutputFilePath)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				slog.Error("error occurred while creating folder", slog.String("path", dir), slog.String("error", err.Error()))
				return
			}
		}
		// Open file
		fd, err = os.OpenFile(s.OutputFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			slog.Error("error occurred while opening file", slog.String("path", s.OutputFilePath), slog.String("error", err.Error()))
			return
		}
		defer func() {
			if closeErr := fd.(*os.File).Close(); closeErr != nil {
				slog.Error("error occurred while closing file", slog.String("path", s.OutputFilePath), slog.String("error", closeErr.Error()))
			}
		}()
	}

	for result := range s.LogChan {
		if _, err := fd.Write([]byte(result + "\n")); err != nil {
			slog.Error("error occurred while writing to file", slog.String("error", err.Error()))
			continue
		}
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
