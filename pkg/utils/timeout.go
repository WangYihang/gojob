package utils

import (
	"context"
)

// RunWithTimeout runs f with the given context and returns as soon as either f
// completes or the context is cancelled (e.g. because it timed out).
//
// The context is passed to f so that well-behaved tasks can abort their work
// when it is cancelled. Note that if f ignores the context, the goroutine
// running it keeps executing in the background until f returns on its own; the
// caller is simply no longer blocked waiting for it.
func RunWithTimeout(ctx context.Context, f func(context.Context) error) error {
	done := make(chan error, 1)

	go func() {
		done <- f(ctx)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}
