package gojob

import (
	"sync"
	"sync/atomic"
	"time"
)

// statusInterval is how often the status manager pushes a status snapshot to
// its subscribers.
const statusInterval = 5 * time.Second

// Status represents the status of the job.
type Status struct {
	Timestamp   string `json:"timestamp"`
	NumFailed   int64  `json:"num_failed"`
	NumSucceed  int64  `json:"num_succeed"`
	NumFinished int64  `json:"num_finished"`
	NumTotal    int64  `json:"num_total"`
}

type statusManager struct {
	numFailed  atomic.Int64
	numSucceed atomic.Int64
	numTotal   atomic.Int64

	mutex       sync.Mutex
	ticker      *time.Ticker
	statusChans []chan Status

	done    chan struct{}
	stopped chan struct{}
}

func newStatusManager() *statusManager {
	return &statusManager{
		numFailed:   atomic.Int64{},
		numSucceed:  atomic.Int64{},
		numTotal:    atomic.Int64{},
		mutex:       sync.Mutex{},
		ticker:      time.NewTicker(statusInterval),
		statusChans: []chan Status{},
		done:        make(chan struct{}),
		stopped:     make(chan struct{}),
	}
}

func (sm *statusManager) notify() {
	status := sm.Snapshot()
	sm.mutex.Lock()
	for _, ch := range sm.statusChans {
		ch <- status
	}
	sm.mutex.Unlock()
}

// Start starts the status manager.
// It notifies all the status channels immediately and then every statusInterval
// until Stop is called.
func (sm *statusManager) Start() {
	defer close(sm.stopped)
	sm.notify()
	for {
		select {
		case <-sm.ticker.C:
			sm.notify()
		case <-sm.done:
			return
		}
	}
}

// Stop stops the status manager.
// It stops the ticker, sends one final status snapshot, and closes every status
// channel so that downstream recorders can finish.
func (sm *statusManager) Stop() {
	// Ask the Start loop to exit and wait for it, so that no notify runs
	// concurrently with the final flush and channel close below.
	close(sm.done)
	<-sm.stopped
	sm.ticker.Stop()
	// Final snapshot so subscribers observe the terminal counts.
	sm.notify()
	sm.mutex.Lock()
	for _, ch := range sm.statusChans {
		close(ch)
	}
	sm.statusChans = nil
	sm.mutex.Unlock()
}

// IncFailed increments the number of failed jobs.
func (sm *statusManager) IncFailed() {
	sm.numFailed.Add(1)
}

// IncSucceed increments the number of succeed jobs.
func (sm *statusManager) IncSucceed() {
	sm.numSucceed.Add(1)
}

// SetTotal sets the total number of jobs.
// It should be called before the job starts.
func (sm *statusManager) SetTotal(total int64) {
	sm.numTotal.Store(total)
}

// StatusChan returns a channel that will receive the status of the job.
// The status will be sent every statusInterval. It should be called before the
// job starts. You can call it multiple times to get multiple channels.
func (sm *statusManager) StatusChan() <-chan Status {
	ch := make(chan Status)
	sm.mutex.Lock()
	sm.statusChans = append(sm.statusChans, ch)
	sm.mutex.Unlock()
	return ch
}

// Snapshot returns the current status of the job.
func (sm *statusManager) Snapshot() Status {
	numFailed := sm.numFailed.Load()
	numSucceed := sm.numSucceed.Load()
	numTotal := sm.numTotal.Load()
	return Status{
		Timestamp:   time.Now().Format(time.RFC3339),
		NumFailed:   numFailed,
		NumSucceed:  numSucceed,
		NumFinished: numFailed + numSucceed,
		NumTotal:    numTotal,
	}
}
