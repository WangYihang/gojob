package pipeline

import "time"

type config struct {
	workers int
	retries int
	backoff BackoffFunc
	timeout time.Duration
}

func defaults() config {
	return config{workers: 1, retries: 1}
}

// Option configures Process (and Execute).
type Option func(*config)

// Workers sets the number of concurrent workers (default 1). Non-positive
// values are ignored.
func Workers(n int) Option {
	return func(c *config) {
		if n > 0 {
			c.workers = n
		}
	}
}

// Timeout bounds the runtime of a single attempt; zero (the default) means no
// timeout. When an attempt exceeds it the attempt's context is cancelled. A
// task that ignores its context keeps running in the background until it
// returns, but the worker stops waiting for it.
func Timeout(d time.Duration) Option {
	return func(c *config) {
		c.timeout = d
	}
}

// Retry sets the maximum number of attempts per item (>= 1) and the backoff
// applied between them. maxAttempts of 1 means a single attempt with no retry.
func Retry(maxAttempts int, backoff BackoffFunc) Option {
	return func(c *config) {
		if maxAttempts > 0 {
			c.retries = maxAttempts
		}
		c.backoff = backoff
	}
}

// BackoffFunc returns how long to wait before a given retry attempt, where
// attempt 1 is the delay before the second overall attempt.
type BackoffFunc func(attempt int) time.Duration

// ExpBackoff grows the delay exponentially from base, capped at max.
func ExpBackoff(base, max time.Duration) BackoffFunc {
	return func(attempt int) time.Duration {
		d := base << uint(attempt-1)
		if d <= 0 || d > max {
			return max
		}
		return d
	}
}

// NoBackoff retries immediately.
func NoBackoff() BackoffFunc {
	return func(int) time.Duration { return 0 }
}
