package gojob_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/WangYihang/gojob"
)

func TestProcessBasic(t *testing.T) {
	ctx := context.Background()
	src := gojob.From(ctx, 1, 2, 3, 4, 5)
	results := gojob.Process(ctx, src, func(ctx context.Context, n int) (int, error) {
		return n * n, nil
	}, gojob.WithWorkers(3))

	got := map[int]bool{}
	for _, r := range collect(results) {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
		if r.Attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", r.Attempts)
		}
		got[r.Value] = true
	}
	for _, want := range []int{1, 4, 9, 16, 25} {
		if !got[want] {
			t.Errorf("missing result %d", want)
		}
	}
	if len(got) != 5 {
		t.Errorf("expected 5 results, got %d", len(got))
	}
}

func TestExecute(t *testing.T) {
	ctx := context.Background()
	tasks := gojob.From[gojob.Task[int]](ctx,
		gojob.TaskFunc[int](func(context.Context) (int, error) { return 10, nil }),
		gojob.TaskFunc[int](func(context.Context) (int, error) { return 20, nil }),
	)
	sum := 0
	for _, r := range collect(gojob.Execute(ctx, tasks)) {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
		sum += r.Value
	}
	if sum != 30 {
		t.Errorf("expected sum 30, got %d", sum)
	}
}

func TestProcessRetrySucceeds(t *testing.T) {
	ctx := context.Background()
	var mu sync.Mutex
	tries := map[int]int{}
	src := gojob.From(ctx, 1, 2, 3)
	results := gojob.Process(ctx, src, func(ctx context.Context, n int) (int, error) {
		mu.Lock()
		tries[n]++
		attempt := tries[n]
		mu.Unlock()
		if attempt < 3 {
			return 0, fmt.Errorf("transient")
		}
		return n, nil
	}, gojob.WithWorkers(2), gojob.WithRetry(5, gojob.NoBackoff()))

	for _, r := range collect(results) {
		if r.Err != nil {
			t.Errorf("expected success after retries, got %v", r.Err)
		}
		if r.Attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", r.Attempts)
		}
	}
}

func TestProcessRetryExhausted(t *testing.T) {
	ctx := context.Background()
	src := gojob.From(ctx, 1, 2)
	results := gojob.Process(ctx, src, func(ctx context.Context, n int) (int, error) {
		return 0, fmt.Errorf("always fails")
	}, gojob.WithRetry(3, gojob.NoBackoff()))

	for _, r := range collect(results) {
		if r.Err == nil {
			t.Error("expected error after exhausting retries")
		}
		if r.Attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", r.Attempts)
		}
	}
}

func TestProcessRetryBackoffCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	src := gojob.From(ctx, 1)
	results := gojob.Process(ctx, src, func(ctx context.Context, n int) (int, error) {
		return 0, fmt.Errorf("fail")
	}, gojob.WithRetry(5, gojob.ExpBackoff(1*time.Second, 1*time.Second)))

	// Cancel while the worker is sleeping between attempts. Without interruptible
	// backoff, 5 attempts would take ~4s; cancellation must cut that short. (The
	// in-flight result may or may not be delivered once ctx is done, so we assert
	// on prompt termination rather than on the number of results.)
	time.AfterFunc(50*time.Millisecond, cancel)
	start := time.Now()
	collect(results)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("expected cancellation to interrupt backoff quickly, took %s", elapsed)
	}
}

func TestProcessTimeout(t *testing.T) {
	ctx := context.Background()
	src := gojob.From(ctx, 1)
	results := gojob.Process(ctx, src, func(ctx context.Context, n int) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(2 * time.Second):
			return n, nil
		}
	}, gojob.WithTimeout(50*time.Millisecond))

	rs := collect(results)
	if len(rs) != 1 {
		t.Fatalf("expected 1 result, got %d", len(rs))
	}
	if !errors.Is(rs[0].Err, context.DeadlineExceeded) {
		t.Errorf("expected deadline exceeded, got %v", rs[0].Err)
	}
}

func TestWorkersConcurrency(t *testing.T) {
	const n = 5
	ctx := context.Background()
	var running sync.WaitGroup
	running.Add(n)
	release := make(chan struct{})
	src := gojob.From(ctx, rangeInts(n)...)
	results := gojob.Process(ctx, src, func(ctx context.Context, i int) (int, error) {
		running.Done() // signal this item is being worked on
		<-release      // block until every worker is confirmed running
		return i, nil
	}, gojob.WithWorkers(n))

	allRunning := make(chan struct{})
	go func() { running.Wait(); close(allRunning) }()
	select {
	case <-allRunning:
		close(release)
	case <-time.After(5 * time.Second):
		t.Fatal("expected n items to run concurrently; WithWorkers(n) not applied?")
	}
	if len(collect(results)) != n {
		t.Errorf("expected %d results", n)
	}
}

func TestProcessCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	src := gojob.From(ctx, rangeInts(10000)...)
	results := gojob.Process(ctx, src, func(ctx context.Context, n int) (int, error) {
		time.Sleep(5 * time.Millisecond)
		return n, nil
	}, gojob.WithWorkers(4))

	time.AfterFunc(30*time.Millisecond, cancel)

	done := make(chan int, 1)
	go func() {
		count := 0
		for range results {
			count++
		}
		done <- count
	}()
	select {
	case count := <-done:
		if count >= 10000 {
			t.Errorf("expected early termination, processed all %d", count)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Process did not terminate after cancellation")
	}
}
