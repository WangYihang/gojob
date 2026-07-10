// Package memq is an in-memory queue.Queue for tests and single-process batch
// jobs. It supports redelivery and dead-lettering but not crash-resume (its
// state dies with the process); use queue/fileq for durability.
package memq

import (
	"context"
	"sync"

	"github.com/WangYihang/gojob/queue"
)

type item[T any] struct {
	payload    T
	deliveries int
}

// Queue is an in-memory queue.Queue[T].
type Queue[T any] struct {
	mu          sync.Mutex
	pending     []item[T]
	outstanding int // pending + delivered-but-not-finalized
	sealed      bool
	closed      bool
	dead        []T
	notify      chan struct{}
}

// New creates an empty in-memory queue.
func New[T any]() *Queue[T] {
	return &Queue[T]{notify: make(chan struct{}, 1)}
}

func (q *Queue[T]) signal() {
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// Publish enqueues a payload.
func (q *Queue[T]) Publish(_ context.Context, payload T) error {
	q.mu.Lock()
	if q.sealed || q.closed {
		q.mu.Unlock()
		return queue.ErrClosed
	}
	q.pending = append(q.pending, item[T]{payload: payload})
	q.outstanding++
	q.mu.Unlock()
	q.signal()
	return nil
}

// Seal marks the queue as complete: once it drains (nothing pending and nothing
// outstanding) Receive's channel closes. Call it after publishing a batch.
func (q *Queue[T]) Seal() {
	q.mu.Lock()
	q.sealed = true
	q.mu.Unlock()
	q.signal()
}

// Close stops delivery and closes Receive channels.
func (q *Queue[T]) Close() error {
	q.mu.Lock()
	q.closed = true
	q.mu.Unlock()
	q.signal()
	return nil
}

// Dead returns a copy of the dead-lettered payloads.
func (q *Queue[T]) Dead() []T {
	q.mu.Lock()
	defer q.mu.Unlock()
	return append([]T(nil), q.dead...)
}

func (q *Queue[T]) drainedLocked() bool {
	return q.sealed && q.outstanding == 0 && len(q.pending) == 0
}

// Receive streams messages until the queue drains, is closed, or ctx is done.
func (q *Queue[T]) Receive(ctx context.Context) (<-chan *queue.Message[T], error) {
	out := make(chan *queue.Message[T])
	go func() {
		defer close(out)
		for {
			q.mu.Lock()
			for len(q.pending) == 0 {
				if q.closed || q.drainedLocked() {
					q.mu.Unlock()
					return
				}
				q.mu.Unlock()
				select {
				case <-q.notify:
				case <-ctx.Done():
					return
				}
				q.mu.Lock()
			}
			it := q.pending[0]
			q.pending = q.pending[1:]
			it.deliveries++
			q.mu.Unlock()

			select {
			case out <- q.wrap(it):
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

func (q *Queue[T]) wrap(it item[T]) *queue.Message[T] {
	var once sync.Once
	finalize := func(action int) func(context.Context) error { // 0 ack, 1 nack, 2 dead
		return func(context.Context) error {
			once.Do(func() {
				q.mu.Lock()
				switch action {
				case 1: // requeue for redelivery
					q.pending = append(q.pending, it)
				default: // ack or dead-letter
					q.outstanding--
					if action == 2 {
						q.dead = append(q.dead, it.payload)
					}
				}
				q.mu.Unlock()
				q.signal()
			})
			return nil
		}
	}
	return queue.NewMessage(it.payload, it.deliveries, queue.Handlers{
		Ack:        finalize(0),
		Nack:       finalize(1),
		DeadLetter: finalize(2),
	})
}
