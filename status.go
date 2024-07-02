package gojob

import (
	"sync"
	"sync/atomic"
	"time"
)

// Status represents the status of the job.
type Status struct {
	Timestamp   string `json:"timestamp"`
	NumFailed   int64  `json:"num_failed"`
	NumSucceed  int64  `json:"num_succeed"`
	NumFinished int64  `json:"num_done"`
	NumTotal    int64  `json:"num_total"`
}

type statusManager struct {
	numFailed  atomic.Int64
	numSucceed atomic.Int64
	numTotal   atomic.Int64

	mutex       sync.Mutex
	ticker      *time.Ticker
	statusChans []chan Status
}

func newStatusManager() *statusManager {
	return &statusManager{
		numFailed:   atomic.Int64{},
		numSucceed:  atomic.Int64{},
		numTotal:    atomic.Int64{},
		mutex:       sync.Mutex{},
		ticker:      time.NewTicker(5 * time.Second),
		statusChans: []chan Status{},
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
// It will notify all the status channels every second.
func (sm *statusManager) Start() {
	sm.notify()
	for range sm.ticker.C {
		sm.notify()
	}
}

// Stop stops the status manager.
func (sm *statusManager) Stop() {
	sm.notify()
	sm.notify()
	sm.ticker.Stop()
	for _, ch := range sm.statusChans {
		close(ch)
	}
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
// The status will be sent every second. It should be called before the job starts.
// You can call it multiple times to get multiple channels.
func (sm *statusManager) StatusChan() <-chan Status {
	ch := make(chan Status)
	sm.mutex.Lock()
	sm.statusChans = append(sm.statusChans, ch)
	sm.mutex.Unlock()
	return ch
}

// Snapshot returns the current status of the job.
func (sm *statusManager) Snapshot() Status {
	sm.mutex.Lock()
	numFailed := sm.numFailed.Load()
	numSucceed := sm.numSucceed.Load()
	numTotal := sm.numTotal.Load()
	sm.mutex.Unlock()
	return Status{
		Timestamp:   time.Now().Format(time.RFC3339),
		NumFailed:   numFailed,
		NumSucceed:  numSucceed,
		NumFinished: numFailed + numSucceed,
		NumTotal:    numTotal,
	}
}
