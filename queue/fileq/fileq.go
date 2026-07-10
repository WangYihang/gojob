// Package fileq is a durable, file-backed queue.Queue. It survives process
// crashes: a message being processed is leased in a "processing" directory, and
// if it is not acknowledged before the lease expires it is returned to
// "pending" and redelivered — so a restarted (or another) consumer resumes it.
//
// Multiple consumers on the same host may share a directory: claiming a message
// is an atomic rename, so two consumers never take the same one.
package fileq

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/WangYihang/gojob/queue"
)

const (
	pendingDir    = "pending"
	processingDir = "processing"
	deadDir       = "dead"
	sealedFile    = "sealed"
)

type config struct {
	lease time.Duration
	poll  time.Duration
}

// Option configures a Queue.
type Option func(*config)

// WithLease sets how long a claimed message stays invisible before it is
// considered abandoned and redelivered (default 30s).
func WithLease(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.lease = d
		}
	}
}

// WithPollInterval sets how often Receive polls for work when idle (default 100ms).
func WithPollInterval(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.poll = d
		}
	}
}

// Queue is a durable file-backed queue.Queue[T].
type Queue[T any] struct {
	dir     string
	codec   queue.Codec[T]
	lease   time.Duration
	poll    time.Duration
	startNS int64
	seq     atomic.Int64
}

// Open creates (if needed) and opens a queue rooted at dir. Payloads are encoded
// as JSON.
func Open[T any](dir string, opts ...Option) (*Queue[T], error) {
	cfg := config{lease: 30 * time.Second, poll: 100 * time.Millisecond}
	for _, o := range opts {
		o(&cfg)
	}
	for _, sub := range []string{pendingDir, processingDir, deadDir} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			return nil, err
		}
	}
	return &Queue[T]{
		dir:     dir,
		codec:   queue.JSONCodec[T]{},
		lease:   cfg.lease,
		poll:    cfg.poll,
		startNS: time.Now().UnixNano(),
	}, nil
}

// Close is a no-op: the queue's state lives on disk.
func (q *Queue[T]) Close() error { return nil }

// Publish writes a new message to the pending directory.
func (q *Queue[T]) Publish(_ context.Context, payload T) error {
	b, err := q.codec.Marshal(payload)
	if err != nil {
		return err
	}
	name := entryName(q.newSeq(), 0, 0)
	tmp := filepath.Join(q.dir, pendingDir, name+".tmp")
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(q.dir, pendingDir, name))
}

// Seal marks the queue as complete so Receive drains and closes once pending and
// processing are both empty. Call it after publishing a batch.
func (q *Queue[T]) Seal() error {
	f, err := os.Create(filepath.Join(q.dir, sealedFile))
	if err != nil {
		return err
	}
	return f.Close()
}

// DeadCount reports how many messages are in the dead-letter directory.
func (q *Queue[T]) DeadCount() int { return q.count(deadDir) }

// Receive streams messages, reclaiming expired leases as it goes.
func (q *Queue[T]) Receive(ctx context.Context) (<-chan *queue.Message[T], error) {
	out := make(chan *queue.Message[T])
	go func() {
		defer close(out)
		for {
			if ctx.Err() != nil {
				return
			}
			q.reclaimExpired()
			m, ok := q.claim()
			if !ok {
				if q.drained() {
					return
				}
				select {
				case <-time.After(q.poll):
				case <-ctx.Done():
					return
				}
				continue
			}
			select {
			case out <- m:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// claim atomically moves the oldest pending entry into processing and wraps it.
func (q *Queue[T]) claim() (*queue.Message[T], bool) {
	names := q.list(pendingDir)
	sort.Strings(names) // rough FIFO by sequence
	for _, n := range names {
		seq, deliveries, _, ok := parseName(n)
		if !ok {
			continue
		}
		deliveries++
		leaseNS := time.Now().Add(q.lease).UnixNano()
		procName := entryName(seq, deliveries, leaseNS)
		src := filepath.Join(q.dir, pendingDir, n)
		dst := filepath.Join(q.dir, processingDir, procName)
		if err := os.Rename(src, dst); err != nil {
			continue // another consumer claimed it first
		}
		b, err := os.ReadFile(dst)
		if err != nil {
			q.divert(procName, seq, deliveries)
			continue
		}
		payload, err := q.codec.Unmarshal(b)
		if err != nil {
			q.divert(procName, seq, deliveries)
			continue
		}
		return q.wrap(seq, deliveries, procName, payload), true
	}
	return nil, false
}

func (q *Queue[T]) wrap(seq string, deliveries int, procName string, payload T) *queue.Message[T] {
	proc := filepath.Join(q.dir, processingDir, procName)
	var once sync.Once
	do := func(fn func() error) error {
		var err error
		once.Do(func() { err = fn() })
		return err
	}
	return queue.NewMessage(payload, deliveries, queue.Handlers{
		Ack: func(context.Context) error {
			return do(func() error { return os.Remove(proc) })
		},
		Nack: func(context.Context) error {
			return do(func() error {
				return os.Rename(proc, filepath.Join(q.dir, pendingDir, entryName(seq, deliveries, 0)))
			})
		},
		DeadLetter: func(context.Context) error {
			return do(func() error {
				return os.Rename(proc, filepath.Join(q.dir, deadDir, entryName(seq, deliveries, 0)))
			})
		},
	})
}

// reclaimExpired returns processing entries whose lease has passed to pending,
// keeping the delivery count (a delivery whose consumer died still counts).
func (q *Queue[T]) reclaimExpired() {
	now := time.Now().UnixNano()
	for _, n := range q.list(processingDir) {
		seq, deliveries, leaseNS, ok := parseName(n)
		if !ok || leaseNS >= now {
			continue
		}
		_ = os.Rename(
			filepath.Join(q.dir, processingDir, n),
			filepath.Join(q.dir, pendingDir, entryName(seq, deliveries, 0)),
		)
	}
}

// divert moves an undecodable processing entry straight to the dead directory.
func (q *Queue[T]) divert(procName, seq string, deliveries int) {
	_ = os.Rename(
		filepath.Join(q.dir, processingDir, procName),
		filepath.Join(q.dir, deadDir, entryName(seq, deliveries, 0)),
	)
}

func (q *Queue[T]) drained() bool {
	// Check processing before pending: a nack renames an entry processing->pending
	// atomically, so this order never observes a message as absent from both.
	return q.sealed() && q.count(processingDir) == 0 && q.count(pendingDir) == 0
}

func (q *Queue[T]) sealed() bool {
	_, err := os.Stat(filepath.Join(q.dir, sealedFile))
	return err == nil
}

func (q *Queue[T]) newSeq() string {
	return fmt.Sprintf("%019d_%08d", q.startNS, q.seq.Add(1))
}

func (q *Queue[T]) list(sub string) []string {
	es, _ := os.ReadDir(filepath.Join(q.dir, sub))
	names := make([]string, 0, len(es))
	for _, e := range es {
		if strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	return names
}

func (q *Queue[T]) count(sub string) int { return len(q.list(sub)) }

// entryName encodes a queue entry's sequence, delivery count, and lease deadline
// (0 for a pending entry) into a filename.
func entryName(seq string, deliveries int, leaseNS int64) string {
	return fmt.Sprintf("%s.%d.%d.json", seq, deliveries, leaseNS)
}

func parseName(name string) (seq string, deliveries int, leaseNS int64, ok bool) {
	base, found := strings.CutSuffix(name, ".json")
	if !found {
		return "", 0, 0, false
	}
	parts := strings.Split(base, ".")
	if len(parts) != 3 {
		return "", 0, 0, false
	}
	d, err1 := strconv.Atoi(parts[1])
	l, err2 := strconv.ParseInt(parts[2], 10, 64)
	if err1 != nil || err2 != nil {
		return "", 0, 0, false
	}
	return parts[0], d, l, true
}
