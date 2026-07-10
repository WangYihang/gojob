package gojob

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/WangYihang/gojob/pkg/utils"
	"github.com/WangYihang/uio"
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

	numShards     int64
	shard         int64
	numTotalTasks int64

	isStarted    atomic.Bool
	currentIndex atomic.Int64

	statusManager *statusManager

	taskChan      chan *basicTask
	resultChans   []chan *basicTask
	resultChansMu sync.Mutex

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

		numShards:     1,
		shard:         0,
		numTotalTasks: 0,

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
	// Validate cross-option constraints once, after every option has been
	// applied, so that the order in which options are supplied does not matter.
	if svr.shard < 0 || svr.shard >= svr.numShards {
		panic(fmt.Errorf("shard must be within the range [0, numShards), got shard=%d numShards=%d", svr.shard, svr.numShards))
	}

	go svr.statusManager.Start()

	// Result recorder.
	svr.recorderWg.Add(1)
	chanRecorder(svr.resultFilePath, svr.ResultChan(), svr.recorderWg)

	// Status recorder(s). The status is always written to the configured status
	// file; unless that file is already stdout ("-"), the status is additionally
	// streamed to stdout so that progress stays visible.
	svr.recorderWg.Add(1)
	chanRecorder(svr.statusFilePath, svr.StatusChan(), svr.recorderWg)
	if svr.statusFilePath != "-" {
		svr.recorderWg.Add(1)
		chanRecorder("-", svr.StatusChan(), svr.recorderWg)
	}

	// Metadata recorder. The channel is buffered so the send below never blocks
	// on the recorder goroutine (or on a failed file open).
	svr.recorderWg.Add(1)
	metadataChan := make(chan schedulerMetadata, 1)
	chanRecorder(svr.metadataFilePath, metadataChan, svr.recorderWg)
	metadataChan <- svr.metadata
	close(metadataChan)

	if svr.prometheusPushGatewayUrl != "" {
		svr.recorderWg.Add(1)
		prometheusPusher(svr.prometheusPushGatewayUrl, svr.prometheusPushGatewayJob, svr.StatusChan(), svr.recorderWg)
	}
	return svr
}

// WithNumShards sets the number of shards, default is 1 which means no sharding
func WithNumShards(numShards int64) schedulerOption {
	return func(s *Scheduler) error {
		if numShards <= 0 {
			return fmt.Errorf("numShards must be greater than 0")
		}
		s.numShards = numShards
		return nil
	}
}

// WithShard sets the shard (from 0 to NumShards-1)
func WithShard(shard int64) schedulerOption {
	return func(s *Scheduler) error {
		if shard < 0 {
			return fmt.Errorf("shard must be greater than or equal to 0")
		}
		s.shard = shard
		return nil
	}
}

// WithNumWorkers sets the number of workers
func WithNumWorkers(numWorkers int) schedulerOption {
	return func(s *Scheduler) error {
		if numWorkers <= 0 {
			return fmt.Errorf("numWorkers must be greater than 0")
		}
		s.numWorkers = numWorkers
		return nil
	}
}

// WithMaxRetries sets the maximum number of attempts per task (>= 1).
// A value of 1 means the task is attempted once with no retry.
func WithMaxRetries(maxRetries int) schedulerOption {
	return func(s *Scheduler) error {
		if maxRetries <= 0 {
			return fmt.Errorf("maxRetries must be greater than 0")
		}
		s.maxRetries = maxRetries
		return nil
	}
}

// WithMaxRuntimePerTaskSeconds sets the max runtime per task seconds
func WithMaxRuntimePerTaskSeconds(maxRuntimePerTaskSeconds int) schedulerOption {
	return func(s *Scheduler) error {
		if maxRuntimePerTaskSeconds <= 0 {
			return fmt.Errorf("maxRuntimePerTaskSeconds must be greater than 0")
		}
		s.maxRuntimePerTaskSeconds = maxRuntimePerTaskSeconds
		return nil
	}
}

// WithTotalTasks sets the total number of tasks across all shards. The number
// of tasks handled by this shard is derived from it when the scheduler starts.
func WithTotalTasks(numTotalTasks int64) schedulerOption {
	return func(s *Scheduler) error {
		if numTotalTasks < 0 {
			return fmt.Errorf("numTotalTasks must be greater than or equal to 0")
		}
		s.numTotalTasks = numTotalTasks
		return nil
	}
}

// WithMetadata adds metadata
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
	fd, err := uio.Open(path)
	if err != nil {
		slog.Error("error occurred while opening file", slog.String("path", path), slog.String("error", err.Error()))
		// Still drain the channel so that senders never block, and release the
		// wait group so that Wait does not deadlock waiting on this recorder.
		go func() {
			defer wg.Done()
			for range ch {
			}
		}()
		return
	}
	go func() {
		defer wg.Done()
		defer fd.Close()
		encoder := json.NewEncoder(fd)
		for item := range ch {
			if err := encoder.Encode(item); err != nil {
				slog.Error("error occurred while serializing data", slog.String("path", path), slog.String("error", err.Error()))
			}
		}
	}()
}

// ResultChan returns a newly created channel to receive results.
// Every time ResultChan is called, a new channel is created, and the results are
// written to all channels. This is useful for multiple consumers (e.g. writing
// to multiple files). It should be called before the scheduler starts.
func (s *Scheduler) ResultChan() <-chan *basicTask {
	c := make(chan *basicTask)
	s.resultChansMu.Lock()
	s.resultChans = append(s.resultChans, c)
	s.resultChansMu.Unlock()
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
	// Atomically reserve an index so that concurrent Submit calls never hand out
	// the same index or miscount the shard assignment.
	index := s.currentIndex.Add(1) - 1
	if (index % s.numShards) == s.shard {
		s.taskWg.Add(1)
		s.taskChan <- newBasicTask(index, task)
	}
}

// Start starts the scheduler. It is safe to call multiple times; only the first
// call spawns the workers and computes the per-shard task total.
func (s *Scheduler) Start() *Scheduler {
	if s.isStarted.CompareAndSwap(false, true) {
		// Compute the number of tasks handled by this shard from the total.
		tasksForShard := s.numTotalTasks / s.numShards
		if s.shard < s.numTotalTasks%s.numShards {
			tasksForShard++
		}
		s.statusManager.SetTotal(tasksForShard)

		for i := 0; i < s.numWorkers; i++ {
			go s.Worker()
		}
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
	s.resultChansMu.Lock()
	for _, resultChan := range s.resultChans {
		close(resultChan)
	}
	s.resultChansMu.Unlock()
	// Flush and close the status channels
	s.statusManager.Stop()
	// Wait for all recorders to finish
	s.recorderWg.Wait()
}

func (s *Scheduler) Status() Status {
	return s.statusManager.Snapshot()
}

// retryBackoff returns how long to wait before the given retry attempt
// (attempt >= 1). It grows exponentially and is capped.
func retryBackoff(attempt int) time.Duration {
	const base = 100 * time.Millisecond
	const max = 10 * time.Second
	d := base << uint(attempt-1)
	if d <= 0 || d > max {
		return max
	}
	return d
}

// runOnce runs a single attempt of the task with the configured per-task timeout.
func (s *Scheduler) runOnce(task *basicTask) error {
	task.NumTries++
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.maxRuntimePerTaskSeconds)*time.Second)
	defer cancel()
	return utils.RunWithTimeout(ctx, task.Task.Do)
}

// Worker is a worker
func (s *Scheduler) Worker() {
	for task := range s.taskChan {
		// StartedAt marks the beginning of the first attempt; FinishedAt is set
		// once after the final attempt, so the pair spans the full duration
		// (including retries and backoff).
		task.StartedAt = time.Now().UnixMicro()
		for i := 0; i < s.maxRetries; i++ {
			if i > 0 {
				time.Sleep(retryBackoff(i))
			}
			if err := s.runOnce(task); err != nil {
				task.Error = err.Error()
			} else {
				task.Error = ""
				break
			}
		}
		task.FinishedAt = time.Now().UnixMicro()
		// Write result to every registered result channel.
		s.resultChansMu.Lock()
		resultChans := s.resultChans
		s.resultChansMu.Unlock()
		for _, resultChan := range resultChans {
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
