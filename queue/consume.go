package queue

import (
	"context"
	"time"

	"github.com/WangYihang/gojob"
)

type config struct {
	workers       int
	retries       int
	backoff       gojob.BackoffFunc
	timeout       time.Duration
	maxDeliveries int
}

// Option configures Consume.
type Option func(*config)

// WithWorkers sets the number of concurrent workers (default 1).
func WithWorkers(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.workers = n
		}
	}
}

// WithRetry sets the in-memory attempts per delivery (and the backoff between
// them) before a message is nacked for redelivery. maxAttempts of 1 means a
// single attempt per delivery.
func WithRetry(maxAttempts int, backoff gojob.BackoffFunc) Option {
	return func(c *config) {
		if maxAttempts > 0 {
			c.retries = maxAttempts
		}
		c.backoff = backoff
	}
}

// WithTimeout bounds a single attempt; zero (the default) means no timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *config) {
		c.timeout = d
	}
}

// WithMaxDeliveries dead-letters a message once it has been delivered this many
// times without success. Zero (the default) means redeliver forever.
func WithMaxDeliveries(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.maxDeliveries = n
		}
	}
}

// Consume pulls messages from q and runs fn on each with a bounded worker pool,
// acknowledging successes, requeuing failures for redelivery, and dead-lettering
// messages that exceed WithMaxDeliveries. It emits one gojob.Result per
// finalized message (acknowledged or dead-lettered) — redeliveries are silent —
// so the returned stream plugs straight into WithStats, WriteJSONL, web.Serve,
// and so on.
//
// Delivery is at-least-once: a crash between processing and Ack causes
// redelivery, so fn should be idempotent.
func Consume[In, Out any](ctx context.Context, q Queue[In], fn func(context.Context, In) (Out, error), opts ...Option) (<-chan gojob.Result[Out], error) {
	cfg := config{workers: 1, retries: 1}
	for _, o := range opts {
		o(&cfg)
	}

	msgs, err := q.Receive(ctx)
	if err != nil {
		return nil, err
	}

	type outcome struct {
		msg *Message[In]
		out Out
	}
	// The per-attempt timeout is applied inside work (rather than via
	// gojob.WithTimeout) so that the message handle rides through Process even
	// when an attempt times out.
	work := func(ctx context.Context, m *Message[In]) (outcome, error) {
		out, err := runWithTimeout(ctx, cfg.timeout, func(ctx context.Context) (Out, error) {
			return fn(ctx, m.Payload)
		})
		return outcome{msg: m, out: out}, err
	}
	processed := gojob.Process(ctx, msgs, work,
		gojob.WithWorkers(cfg.workers),
		gojob.WithRetry(cfg.retries, cfg.backoff),
	)

	out := make(chan gojob.Result[Out])
	go func() {
		defer close(out)
		for r := range processed {
			oc := r.Value
			m := oc.msg
			switch {
			case r.Err == nil:
				_ = m.Ack(ctx)
			case cfg.maxDeliveries > 0 && m.Deliveries() >= cfg.maxDeliveries:
				_ = m.DeadLetter(ctx)
			default:
				_ = m.Nack(ctx)
				continue // redelivered later; emit a Result only when finalized
			}
			select {
			case out <- gojob.Result[Out]{
				Value:     oc.out,
				Err:       r.Err,
				Attempts:  r.Attempts,
				StartedAt: r.StartedAt,
				Duration:  r.Duration,
			}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// runWithTimeout runs f with a per-attempt timeout, returning the zero value and
// the context error if it elapses. It mirrors the core engine's behaviour but
// lives here so Consume can apply the timeout itself and keep the message handle.
func runWithTimeout[Out any](ctx context.Context, timeout time.Duration, f func(context.Context) (Out, error)) (Out, error) {
	if timeout <= 0 {
		return f(ctx)
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	type res struct {
		v   Out
		err error
	}
	done := make(chan res, 1)
	go func() {
		v, err := f(cctx)
		done <- res{v, err}
	}()
	select {
	case <-cctx.Done():
		var zero Out
		return zero, cctx.Err()
	case r := <-done:
		return r.v, r.err
	}
}
