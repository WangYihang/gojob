package gojob

import (
	"context"
	"fmt"
	"io"
	"sync/atomic"
	"time"
)

// Stats observes a Result stream without altering it. Obtain one from WithStats
// and read it from any number of observers (a progress reporter, a Prometheus
// pusher, ...). It is safe for concurrent use.
type Stats struct {
	total   int64
	done    atomic.Int64
	failed  atomic.Int64
	started time.Time
	fin     chan struct{}
}

// Snapshot is an immutable view of the counters at a point in time.
// Total is -1 when the expected total is unknown (see Total).
type Snapshot struct {
	Total     int64         `json:"total"`
	Done      int64         `json:"done"`
	Succeeded int64         `json:"succeeded"`
	Failed    int64         `json:"failed"`
	Elapsed   time.Duration `json:"elapsed"`
}

// StatsOption configures WithStats.
type StatsOption func(*Stats)

// WithTotal records the expected number of items so snapshots have a denominator.
func WithTotal(n int64) StatsOption {
	return func(s *Stats) { s.total = n }
}

// WithStats returns a pass-through of in that counts results as they flow, plus
// a Stats handle observers can read. The pass-through must be drained (by a
// sink) for the counts to advance and for the stream to be reported complete.
func WithStats[T any](ctx context.Context, in <-chan Result[T], opts ...StatsOption) (<-chan Result[T], *Stats) {
	s := &Stats{total: -1, started: time.Now(), fin: make(chan struct{})}
	for _, o := range opts {
		o(s)
	}
	out := make(chan Result[T])
	go func() {
		defer close(out)
		defer close(s.fin)
		for r := range in {
			s.done.Add(1)
			if r.Err != nil {
				s.failed.Add(1)
			}
			select {
			case out <- r:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, s
}

// Snapshot returns the current counters.
func (s *Stats) Snapshot() Snapshot {
	done := s.done.Load()
	failed := s.failed.Load()
	return Snapshot{
		Total:     s.total,
		Done:      done,
		Succeeded: done - failed,
		Failed:    failed,
		Elapsed:   time.Since(s.started),
	}
}

// Done is closed once the observed stream has ended.
func (s *Stats) Done() <-chan struct{} { return s.fin }

// Stream emits a Snapshot every interval until the observed stream ends, then a
// final Snapshot, then closes. Each call returns an independent channel.
func (s *Stats) Stream(interval time.Duration) <-chan Snapshot {
	ch := make(chan Snapshot, 1)
	go func() {
		defer close(ch)
		t := time.NewTicker(interval)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				ch <- s.Snapshot()
			case <-s.fin:
				ch <- s.Snapshot()
				return
			}
		}
	}()
	return ch
}

// ReportEvery writes a human-readable progress line to w every interval until
// the observed stream ends. Run it in its own goroutine.
func ReportEvery(stats *Stats, interval time.Duration, w io.Writer) {
	for snap := range stats.Stream(interval) {
		if snap.Total >= 0 {
			fmt.Fprintf(w, "progress: %d/%d done, %d ok, %d failed, elapsed %s\n",
				snap.Done, snap.Total, snap.Succeeded, snap.Failed, snap.Elapsed.Round(time.Second))
		} else {
			fmt.Fprintf(w, "progress: %d done, %d ok, %d failed, elapsed %s\n",
				snap.Done, snap.Succeeded, snap.Failed, snap.Elapsed.Round(time.Second))
		}
	}
}
