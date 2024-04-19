package gojob

import (
	"sync"
	"time"
)

type Status struct {
	Timestamp   string `json:"timestamp"`
	NumFailed   int64  `json:"num_failed"`
	NumSucceed  int64  `json:"num_succeed"`
	NumFinished int64  `json:"num_done"`
	NumTotal    int64  `json:"num_total"`
}

type StatusManager struct {
	numFailed  int64
	numSucceed int64
	numTotal   int64

	mutex       *sync.Mutex
	ticker      *time.Ticker
	statusChans []chan Status
}

func NewStatusManager() *StatusManager {
	return &StatusManager{
		mutex:       &sync.Mutex{},
		ticker:      time.NewTicker(1 * time.Second),
		statusChans: []chan Status{},
	}
}

func (sm *StatusManager) Start() {
	for range sm.ticker.C {
		sm.notify()
	}
}

func (sm *StatusManager) Stop() {
	sm.notify()
	sm.ticker.Stop()
	for _, ch := range sm.statusChans {
		close(ch)
	}
}

func (sm *StatusManager) IncFailed() {
	sm.mutex.Lock()
	sm.numFailed++
	sm.mutex.Unlock()
}

func (sm *StatusManager) IncSucceed() {
	sm.mutex.Lock()
	sm.numSucceed++
	sm.mutex.Unlock()
}

func (sm *StatusManager) SetTotal(total int64) {
	sm.mutex.Lock()
	sm.numTotal = total
	sm.mutex.Unlock()
}

func (sm *StatusManager) StatusChan() <-chan Status {
	ch := make(chan Status)
	sm.statusChans = append(sm.statusChans, ch)
	return ch
}

func (sm *StatusManager) Snapshot() Status {
	sm.mutex.Lock()
	status := Status{
		Timestamp:   time.Now().Format(time.RFC3339),
		NumFailed:   sm.numFailed,
		NumSucceed:  sm.numSucceed,
		NumFinished: sm.numFailed + sm.numSucceed,
		NumTotal:    sm.numTotal,
	}
	sm.mutex.Unlock()
	return status
}

func (sm *StatusManager) notify() {
	status := sm.Snapshot()
	for _, ch := range sm.statusChans {
		ch <- status
	}
}
