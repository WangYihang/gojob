package gojob

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/WangYihang/gojob/pkg/utils"
	"github.com/google/uuid"
)

// Scheduler is a task scheduler
type Scheduler struct {
	RunID                    string
	NumWorkers               int
	OutputFilePath           string
	OutputFd                 io.WriteCloser
	StatusFilePath           string
	StatusFd                 io.WriteCloser
	MetadataFilePath         string
	MetadataFd               io.WriteCloser
	Metadata                 map[string]interface{}
	MaxRetries               int
	MaxRuntimePerTaskSeconds int
	NumShards                int64
	Shard                    int64
	IsStarted                bool
	CurrentIndex             atomic.Int64
	FailedTaskCount          atomic.Int64
	SucceedTaskCount         atomic.Int64
	TotalTaskCount           atomic.Int64
	TaskChan                 chan *BasicTask
	LogChan                  chan string
	DoneChan                 chan struct{}
	taskWg                   *sync.WaitGroup
	logWg                    *sync.WaitGroup
	statusWg                 *sync.WaitGroup
}

// NewScheduler creates a new scheduler
func NewScheduler() *Scheduler {
	id := uuid.New().String()
	return (&Scheduler{
		RunID:                    id,
		NumWorkers:               1,
		Metadata:                 make(map[string]interface{}),
		MaxRetries:               4,
		MaxRuntimePerTaskSeconds: 16,
		NumShards:                1,
		Shard:                    0,
		IsStarted:                false,
		CurrentIndex:             atomic.Int64{},
		SucceedTaskCount:         atomic.Int64{},
		TotalTaskCount:           atomic.Int64{},
		TaskChan:                 make(chan *BasicTask),
		LogChan:                  make(chan string),
		DoneChan:                 make(chan struct{}),
		taskWg:                   &sync.WaitGroup{},
		logWg:                    &sync.WaitGroup{},
		statusWg:                 &sync.WaitGroup{},
	}).
		SetOutputFilePath("-").
		SetStatusFilePath("-").
		SetMetadataFilePath("-").
		SetMetadata("id", id)
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
	fd, err := FilePathToFd(s.OutputFilePath)
	if err != nil {
		panic(err)
	}
	s.OutputFd = fd
	// Set status file path and metadata file path
	if s.OutputFilePath != "-" && s.OutputFilePath != "" {
		outputFilePathWithoutExt := outputFilePath[:len(outputFilePath)-len(filepath.Ext(outputFilePath))]
		s.SetStatusFilePath(outputFilePathWithoutExt + ".status")
		s.SetMetadataFilePath(outputFilePathWithoutExt + ".metadata")
	}
	return s
}

// SetStatusFilePath sets the status file path
func (s *Scheduler) SetStatusFilePath(statusFilePath string) *Scheduler {
	s.StatusFilePath = statusFilePath
	fd, err := FilePathToFd(s.StatusFilePath)
	if err != nil {
		panic(err)
	}
	s.StatusFd = utils.NewTeeWriterCloser(fd, os.Stderr)
	return s
}

// SetMetadataFilePath sets the metadata file path
func (s *Scheduler) SetMetadataFilePath(metadataFilePath string) *Scheduler {
	s.MetadataFilePath = metadataFilePath
	fd, err := FilePathToFd(s.MetadataFilePath)
	if err != nil {
		panic(err)
	}
	s.MetadataFd = utils.NewTeeWriterCloser(fd, os.Stderr)
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

func (s *Scheduler) SetTotalTasks(numTotalTasks int64) *Scheduler {
	// Check if NumShards is set and is greater than 0
	if s.NumShards <= 0 {
		panic("NumShards must be greater than 0")
	}

	// Check if Shard is set and is within the valid range [0, NumShards)
	if s.Shard < 0 || s.Shard >= s.NumShards {
		panic("Shard must be within the range [0, NumShards)")
	}

	// Calculate the base number of tasks per shard
	baseTasksPerShard := numTotalTasks / int64(s.NumShards)

	// Calculate the remainder
	remainder := numTotalTasks % int64(s.NumShards)

	// Adjust task count for shards that need to handle an extra task due to the remainder
	if int64(s.Shard) < remainder {
		baseTasksPerShard++
	}

	// Store the number of tasks for this shard
	s.TotalTaskCount.Store(baseTasksPerShard)
	return s
}

// AddMetadata adds metadata
func (s *Scheduler) SetMetadata(key string, value interface{}) *Scheduler {
	if s.IsStarted {
		panic("cannot add metadata after starting")
	}
	s.Metadata[key] = value
	return s
}

// Save saves metadata
func (s *Scheduler) Save() {
	data, err := json.Marshal(s.Metadata)
	if err != nil {
		slog.Error("error occured while serializing metadata", slog.String("error", err.Error()))
	} else {
		s.MetadataFd.Write(data)
		s.MetadataFd.Write([]byte("\n"))
	}
}

// Submit submits a task to the scheduler
func (s *Scheduler) Submit(task Task) {
	if !s.IsStarted {
		s.Start()
	}
	index := s.CurrentIndex.Load()
	if (index % s.NumShards) == s.Shard {
		s.taskWg.Add(1)
		s.TaskChan <- NewBasicTask(index, s.RunID, task)
	}
	s.CurrentIndex.Add(1)
}

// Start starts the scheduler
func (s *Scheduler) Start() *Scheduler {
	if s.IsStarted {
		return s
	}
	s.Save()
	for i := 0; i < s.NumWorkers; i++ {
		go s.Worker()
	}
	go s.ResultWriter()
	s.statusWg.Add(1)
	go s.StatusWriter()
	s.IsStarted = true
	return s
}

// Wait waits for all tasks to finish
func (s *Scheduler) Wait() {
	s.taskWg.Wait()
	close(s.TaskChan)
	s.logWg.Wait()
	close(s.LogChan)
	close(s.DoneChan)
	s.statusWg.Wait()
	s.MetadataFd.Close()
}

func (s *Scheduler) Status() *Status {
	return NewStatus(s.FailedTaskCount.Load(), s.SucceedTaskCount.Load(), s.TotalTaskCount.Load())
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
				return utils.RunWithTimeout(task.Task.Do, time.Duration(s.MaxRuntimePerTaskSeconds)*time.Second)
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
			s.FailedTaskCount.Add(1)
		} else {
			s.logWg.Add(1)
			s.LogChan <- string(data)
			s.SucceedTaskCount.Add(1)
		}
		// Notify task is done
		s.taskWg.Done()
	}
}

// ResultWriter writes logs to file
func (s *Scheduler) ResultWriter() {
	defer s.OutputFd.Close()
	for result := range s.LogChan {
		if _, err := s.OutputFd.Write([]byte(result + "\n")); err != nil {
			slog.Error("error occurred while writing to file", slog.String("error", err.Error()))
			continue
		}
		s.logWg.Done()
	}
}

func (s *Scheduler) StatusWriter() {
	tick := time.NewTicker(1 * time.Second)
	defer s.statusWg.Done()
	defer s.StatusFd.Close()
	defer tick.Stop()
	for {
		select {
		case <-s.DoneChan:
			s.StatusFd.Write([]byte(s.Status().String() + "\n"))
			return
		case <-tick.C:
			s.StatusFd.Write([]byte(s.Status().String() + "\n"))
		}
	}
}

type writeCloserWrapper struct {
	io.Writer
}

func (wc writeCloserWrapper) Close() error {
	return nil
}

func FilePathToFd(path string) (io.WriteCloser, error) {
	switch path {
	case "-":
		return os.Stdout, nil
	case "":
		return writeCloserWrapper{io.Discard}, nil
	default:
		// Create folder if not exists
		dir := filepath.Dir(path)
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, err
			}
		}
		// Open file
		fd, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, err
		}
		return fd, nil
	}
}
