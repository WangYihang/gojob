package utils

import (
	"context"
	"time"
)

func RunWithTimeout(f func(ctx context.Context) error, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	done := make(chan error, 1)

	go func() {
		defer close(done)

		done <- f(ctx)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}
