package gojob

import (
	"context"
	"sync"
	"time"
)

// Process runs fn over every item read from in, using a bounded pool of
// workers, and streams a Result for each item on the returned channel.
//
// Results are emitted in completion order, not input order. The returned
// channel is closed once in is drained and all workers finish, or once ctx is
// cancelled. Process does not block the caller; wire the returned channel into
// a sink (e.g. WriteJSONL) to drive it to completion.
func Process[In, Out any](
	ctx context.Context,
	in <-chan In,
	fn func(context.Context, In) (Out, error),
	opts ...Option,
) <-chan Result[Out] {
	cfg := defaults()
	for _, o := range opts {
		o(&cfg)
	}

	out := make(chan Result[Out])
	var wg sync.WaitGroup
	wg.Add(cfg.workers)
	for i := 0; i < cfg.workers; i++ {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case input, ok := <-in:
					if !ok {
						return
					}
					r := runOne(ctx, input, fn, cfg)
					select {
					case out <- r:
					case <-ctx.Done():
						return
					}
				}
			}
		}()
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

// Execute runs a stream of self-contained tasks and streams their results.
// It is a thin adapter over Process for the "one task object per item" style.
func Execute[T any](ctx context.Context, tasks <-chan Task[T], opts ...Option) <-chan Result[T] {
	return Process(ctx, tasks, func(ctx context.Context, t Task[T]) (T, error) {
		return t.Execute(ctx)
	}, opts...)
}

// runOne executes a single item with the configured retry and timeout policy,
// recording the number of attempts and the wall-clock span across them.
func runOne[In, Out any](ctx context.Context, input In, fn func(context.Context, In) (Out, error), cfg config) Result[Out] {
	started := time.Now()
	var (
		val      Out
		err      error
		attempts int
	)
	for attempt := 1; attempt <= cfg.retries; attempt++ {
		if attempt > 1 && cfg.backoff != nil {
			if d := cfg.backoff(attempt - 1); d > 0 {
				timer := time.NewTimer(d)
				select {
				case <-timer.C:
				case <-ctx.Done():
					timer.Stop()
					return result(val, ctx.Err(), attempts, started)
				}
			}
		}
		attempts = attempt
		val, err = runWithTimeout(ctx, input, fn, cfg.timeout)
		if err == nil {
			break
		}
	}
	return result(val, err, attempts, started)
}

func result[Out any](val Out, err error, attempts int, started time.Time) Result[Out] {
	return Result[Out]{
		Value:     val,
		Err:       err,
		Attempts:  attempts,
		StartedAt: started,
		Duration:  time.Since(started),
	}
}

// runWithTimeout runs fn with a per-attempt timeout. When timeout is zero fn is
// called directly. Otherwise fn runs in its own goroutine so that a task which
// ignores its context cannot block the worker past the deadline.
func runWithTimeout[In, Out any](ctx context.Context, input In, fn func(context.Context, In) (Out, error), timeout time.Duration) (Out, error) {
	if timeout <= 0 {
		return fn(ctx, input)
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	type outcome struct {
		val Out
		err error
	}
	done := make(chan outcome, 1)
	go func() {
		val, err := fn(cctx, input)
		done <- outcome{val, err}
	}()
	select {
	case <-cctx.Done():
		var zero Out
		return zero, cctx.Err()
	case o := <-done:
		return o.val, o.err
	}
}
