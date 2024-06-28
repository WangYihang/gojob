package gojob

import (
	"sync"
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
	numFailed  int64
	numSucceed int64
	numTotal   int64

	mutex       *sync.Mutex
	ticker      *time.Ticker
	statusChans []chan Status
}

func newStatusManager() *statusManager {
	return &statusManager{
		mutex:       &sync.Mutex{},
		ticker:      time.NewTicker(5 * time.Second),
		statusChans: []chan Status{},
	}
}

func (sm *statusManager) notify() {
	status := sm.Snapshot()
	for _, ch := range sm.statusChans {
		ch <- status
	}
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
	sm.ticker.Stop()
	for _, ch := range sm.statusChans {
		close(ch)
	}
}

// IncFailed increments the number of failed jobs.
func (sm *statusManager) IncFailed() {
	sm.mutex.Lock()
	sm.numFailed++
	sm.mutex.Unlock()
}

// IncSucceed increments the number of succeed jobs.
func (sm *statusManager) IncSucceed() {
	sm.mutex.Lock()
	sm.numSucceed++
	sm.mutex.Unlock()
}

// SetTotal sets the total number of jobs.
// It should be called before the job starts.
func (sm *statusManager) SetTotal(total int64) {
	sm.mutex.Lock()
	sm.numTotal = total
	sm.mutex.Unlock()
}

// StatusChan returns a channel that will receive the status of the job.
// The status will be sent every second. It should be called before the job starts.
// You can call it multiple times to get multiple channels.
func (sm *statusManager) StatusChan() <-chan Status {
	ch := make(chan Status)
	sm.statusChans = append(sm.statusChans, ch)
	return ch
}

// Snapshot returns the current status of the job.
func (sm *statusManager) Snapshot() Status {
	defer func() func() {
		sm.mutex.Lock()
		return func() { sm.mutex.Unlock() }
	}()()
	status := Status{
		Timestamp:   time.Now().Format(time.RFC3339),
		NumFailed:   sm.numFailed,
		NumSucceed:  sm.numSucceed,
		NumFinished: sm.numFailed + sm.numSucceed,
		NumTotal:    sm.numTotal,
	}
	return status
}
