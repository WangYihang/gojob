package gojob

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/WangYihang/gojob/pkg/utils"
	"github.com/google/uuid"
)

type schedulerMetadata map[string]interface{}

// Scheduler is a task scheduler
type Scheduler struct {
	id         string
	numWorkers int
	metadata   schedulerMetadata

	maxRetries               int
	maxRuntimePerTaskSeconds int

	numShards int64
	shard     int64

	isStarted    atomic.Bool
	currentIndex atomic.Int64

	statusManager *statusManager

	taskChan    chan *basicTask
	resultChans []chan *basicTask

	resultFilePath           string
	statusFilePath           string
	metadataFilePath         string
	prometheusPushGatewayUrl string
	prometheusPushGatewayJob string

	taskWg     *sync.WaitGroup
	recorderWg *sync.WaitGroup
}

type schedulerOption func(*Scheduler) error

func New(options ...schedulerOption) *Scheduler {
	id := uuid.New().String()
	svr := &Scheduler{
		id:         id,
		numWorkers: 1,
		metadata:   schedulerMetadata{"id": id},

		maxRetries:               4,
		maxRuntimePerTaskSeconds: 16,

		numShards: 1,
		shard:     0,

		isStarted:    atomic.Bool{},
		currentIndex: atomic.Int64{},

		statusManager: newStatusManager(),

		taskChan:    make(chan *basicTask),
		resultChans: []chan *basicTask{},

		resultFilePath:   "result.json",
		statusFilePath:   "status.json",
		metadataFilePath: "metadata.json",

		prometheusPushGatewayUrl: "",
		prometheusPushGatewayJob: "gojob",

		taskWg:     &sync.WaitGroup{},
		recorderWg: &sync.WaitGroup{},
	}
	for _, opt := range options {
		err := opt(svr)
		if err != nil {
			panic(err)
		}
	}
	go svr.statusManager.Start()
	svr.recorderWg.Add(3)
	chanRecorder(svr.resultFilePath, svr.ResultChan(), svr.recorderWg)
	chanRecorder(svr.statusFilePath, svr.StatusChan(), svr.recorderWg)
	metadataChan := make(chan schedulerMetadata)
	chanRecorder(svr.metadataFilePath, metadataChan, svr.recorderWg)
	metadataChan <- svr.metadata
	close(metadataChan)
	if svr.prometheusPushGatewayUrl != "" {
		svr.recorderWg.Add(1)
		prometheusPusher(svr.prometheusPushGatewayUrl, svr.prometheusPushGatewayJob, svr.StatusChan(), svr.recorderWg)
	}
	return svr
}

// SetNumShards sets the number of shards, default is 1 which means no sharding
func WithNumShards(numShards int64) schedulerOption {
	return func(s *Scheduler) error {
		if numShards <= 0 {
			return fmt.Errorf("numShards must be greater than 0")
		}
		s.numShards = numShards
		return nil
	}
}

// SetShard sets the shard (from 0 to NumShards-1)
func WithShard(shard int64) schedulerOption {
	return func(s *Scheduler) error {
		if shard < 0 || shard >= s.numShards {
			return fmt.Errorf("shard must be in [0, NumShards)")
		}
		s.shard = shard
		return nil
	}
}

// SetNumWorkers sets the number of workers
func WithNumWorkers(numWorkers int) schedulerOption {
	return func(s *Scheduler) error {
		if numWorkers <= 0 {
			return fmt.Errorf("numWorkers must be greater than 0")
		}
		s.numWorkers = numWorkers
		return nil
	}
}

// SetMaxRetries sets the max retries
func WithMaxRetries(maxRetries int) schedulerOption {
	return func(s *Scheduler) error {
		if maxRetries <= 0 {
			return fmt.Errorf("maxRetries must be greater than 0")
		}
		s.maxRetries = maxRetries
		return nil
	}
}

// SetMaxRuntimePerTaskSeconds sets the max runtime per task seconds
func WithMaxRuntimePerTaskSeconds(maxRuntimePerTaskSeconds int) schedulerOption {
	return func(s *Scheduler) error {
		if maxRuntimePerTaskSeconds <= 0 {
			return fmt.Errorf("maxRuntimePerTaskSeconds must be greater than 0")
		}
		s.maxRuntimePerTaskSeconds = maxRuntimePerTaskSeconds
		return nil
	}
}

// WithTotalTasks sets the total number of tasks, and calculates the number of tasks for this shard
func WithTotalTasks(numTotalTasks int64) schedulerOption {
	return func(s *Scheduler) error {
		// Check if NumShards is set and is greater than 0
		if s.numShards <= 0 {
			return fmt.Errorf("number of shards must be greater than 0")
		}

		// Check if Shard is set and is within the valid range [0, NumShards)
		if s.shard < 0 || s.shard >= s.numShards {
			return fmt.Errorf("shard must be within the range [0, NumShards)")
		}

		// Calculate the base number of tasks per shard
		baseTasksPerShard := numTotalTasks / int64(s.numShards)

		// Calculate the remainder
		remainder := numTotalTasks % int64(s.numShards)

		// Adjust task count for shards that need to handle an extra task due to the remainder
		if int64(s.shard) < remainder {
			baseTasksPerShard++
		}

		// Store the number of tasks for this shard
		s.statusManager.SetTotal(baseTasksPerShard)
		return nil
	}
}

// AddMetadata adds metadata
func WithMetadata(key string, value interface{}) schedulerOption {
	return func(s *Scheduler) error {
		s.metadata[key] = value
		return nil
	}
}

// WithResultFilePath sets the file path for results
func WithResultFilePath(path string) schedulerOption {
	return func(s *Scheduler) error {
		s.resultFilePath = path
		return nil
	}
}

// WithStatusFilePath sets the file path for status
func WithStatusFilePath(path string) schedulerOption {
	return func(s *Scheduler) error {
		s.statusFilePath = path
		return nil
	}
}

func WithPrometheusPushGateway(url string, job string) schedulerOption {
	return func(s *Scheduler) error {
		s.prometheusPushGatewayUrl = url
		s.prometheusPushGatewayJob = job
		return nil
	}
}

// WithMetadataFilePath sets the file path for metadata
func WithMetadataFilePath(path string) schedulerOption {
	return func(s *Scheduler) error {
		s.metadataFilePath = path
		return nil
	}
}

// chanRecorder records the channel to a given file path
func chanRecorder[T *basicTask | Status | schedulerMetadata](path string, ch <-chan T, wg *sync.WaitGroup) {
	fd, err := utils.OpenFile(path)
	if err != nil {
		slog.Error("error occured while opening file", slog.String("path", path), slog.String("error", err.Error()))
		return
	}
	go func() {
		defer fd.Close()
		encoder := json.NewEncoder(fd)
		for item := range ch {
			if err := encoder.Encode(item); err != nil {
				slog.Error("error occured while serializing data", slog.String("path", path), slog.String("error", err.Error()))
			}
		}
		wg.Done()
	}()
}

// ResultChan returns a newly created channel to receive results
// Everytime ResultChan is called, a new channel is created, and the results are written to all channels
// This is useful for multiple consumers (e.g. writing to multiple files)
func (s *Scheduler) ResultChan() <-chan *basicTask {
	c := make(chan *basicTask)
	s.resultChans = append(s.resultChans, c)
	return c
}

// StatusChan returns a newly created channel to receive status
// Everytime StatusChan is called, a new channel is created, and the status are written to all channels
// This is useful for multiple consumers (e.g. writing to multiple files, report to prometheus, etc.)
func (s *Scheduler) StatusChan() <-chan Status {
	return s.statusManager.StatusChan()
}

func (s *Scheduler) Metadata() map[string]interface{} {
	return s.metadata
}

// Submit submits a task to the scheduler
func (s *Scheduler) Submit(task Task) {
	if !s.isStarted.Load() {
		s.Start()
	}
	index := s.currentIndex.Load()
	if (index % s.numShards) == s.shard {
		s.taskWg.Add(1)
		s.taskChan <- newBasicTask(index, s.id, task)
	}
	s.currentIndex.Add(1)
}

// Start starts the scheduler
func (s *Scheduler) Start() *Scheduler {
	for i := 0; i < s.numWorkers; i++ {
		go s.Worker()
	}
	return s
}

// Wait waits for all tasks to finish
func (s *Scheduler) Wait() {
	// Wait for all tasks to finish
	s.taskWg.Wait()
	// Close task channel
	close(s.taskChan)
	// Close result channels
	for _, resultChan := range s.resultChans {
		close(resultChan)
	}
	// Wait for all recorders to finish
	s.statusManager.Stop()
	// Wait for all recorders to finish
	s.recorderWg.Wait()
}

func (s *Scheduler) Status() Status {
	return s.statusManager.Snapshot()
}

// Worker is a worker
func (s *Scheduler) Worker() {
	for task := range s.taskChan {
		// Start task
		for i := 0; i < s.maxRetries; i++ {
			err := func() error {
				task.StartedAt = time.Now().UnixMicro()
				defer func() {
					task.NumTries++
					task.FinishedAt = time.Now().UnixMicro()
				}()
				return utils.RunWithTimeout(task.Task.Do, time.Duration(s.maxRuntimePerTaskSeconds)*time.Second)
			}()
			if err != nil {
				task.Error = err.Error()
			} else {
				task.Error = ""
				break
			}
		}
		// Write log
		for _, resultChan := range s.resultChans {
			resultChan <- task
		}
		// Update status
		if task.Error != "" {
			s.statusManager.IncFailed()
		} else {
			s.statusManager.IncSucceed()
		}
		// Notify task is done
		s.taskWg.Done()
	}
}
