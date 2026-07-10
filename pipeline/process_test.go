package pipeline_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/WangYihang/gojob/pipeline"
)

func collect[T any](in <-chan pipeline.Result[T]) []pipeline.Result[T] {
	var out []pipeline.Result[T]
	for r := range in {
		out = append(out, r)
	}
	return out
}

func TestProcessBasic(t *testing.T) {
	ctx := context.Background()
	src := pipeline.From(ctx, 1, 2, 3, 4, 5)
	results := pipeline.Process(ctx, src, func(ctx context.Context, n int) (int, error) {
		return n * n, nil
	}, pipeline.Workers(3))

	got := map[int]bool{}
	for _, r := range collect(results) {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
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
	tasks := pipeline.From[pipeline.Task[int]](ctx,
		pipeline.TaskFunc[int](func(context.Context) (int, error) { return 10, nil }),
		pipeline.TaskFunc[int](func(context.Context) (int, error) { return 20, nil }),
	)
	sum := 0
	for _, r := range collect(pipeline.Execute(ctx, tasks)) {
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
	src := pipeline.From(ctx, 1, 2, 3)
	results := pipeline.Process(ctx, src, func(ctx context.Context, n int) (int, error) {
		mu.Lock()
		tries[n]++
		attempt := tries[n]
		mu.Unlock()
		if attempt < 3 {
			return 0, fmt.Errorf("transient")
		}
		return n, nil
	}, pipeline.Workers(2), pipeline.Retry(5, pipeline.NoBackoff()))

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
	src := pipeline.From(ctx, 1, 2)
	results := pipeline.Process(ctx, src, func(ctx context.Context, n int) (int, error) {
		return 0, fmt.Errorf("always fails")
	}, pipeline.Retry(3, pipeline.NoBackoff()))

	for _, r := range collect(results) {
		if r.Err == nil {
			t.Error("expected error after exhausting retries")
		}
		if r.Attempts != 3 {
			t.Errorf("expected 3 attempts, got %d", r.Attempts)
		}
	}
}

func TestProcessTimeout(t *testing.T) {
	ctx := context.Background()
	src := pipeline.From(ctx, 1)
	results := pipeline.Process(ctx, src, func(ctx context.Context, n int) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(2 * time.Second):
			return n, nil
		}
	}, pipeline.Timeout(50*time.Millisecond))

	rs := collect(results)
	if len(rs) != 1 {
		t.Fatalf("expected 1 result, got %d", len(rs))
	}
	if !errors.Is(rs[0].Err, context.DeadlineExceeded) {
		t.Errorf("expected deadline exceeded, got %v", rs[0].Err)
	}
}

func TestProcessCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	nums := make([]int, 10000)
	for i := range nums {
		nums[i] = i
	}
	src := pipeline.From(ctx, nums...)
	results := pipeline.Process(ctx, src, func(ctx context.Context, n int) (int, error) {
		time.Sleep(5 * time.Millisecond)
		return n, nil
	}, pipeline.Workers(4))

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
