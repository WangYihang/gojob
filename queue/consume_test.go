package queue_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/queue"
	"github.com/WangYihang/gojob/queue/memq"
)

func TestConsumeSuccess(t *testing.T) {
	ctx := context.Background()
	q := memq.New[int]()
	for i := 0; i < 10; i++ {
		if err := q.Publish(ctx, i); err != nil {
			t.Fatal(err)
		}
	}
	q.Seal()

	results, err := queue.Consume(ctx, q, func(ctx context.Context, n int) (int, error) {
		return n * n, nil
	}, queue.WithWorkers(4))
	if err != nil {
		t.Fatal(err)
	}

	got := map[int]bool{}
	for r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
		got[r.Value] = true
	}
	if len(got) != 10 {
		t.Errorf("expected 10 results, got %d", len(got))
	}
	if d := q.Dead(); len(d) != 0 {
		t.Errorf("expected nothing dead-lettered, got %v", d)
	}
}

func TestConsumeRetryThenAck(t *testing.T) {
	ctx := context.Background()
	q := memq.New[int]()
	_ = q.Publish(ctx, 1)
	q.Seal()

	var mu sync.Mutex
	attempts := 0
	results, _ := queue.Consume(ctx, q, func(ctx context.Context, n int) (int, error) {
		mu.Lock()
		attempts++
		a := attempts
		mu.Unlock()
		if a < 3 {
			return 0, fmt.Errorf("transient")
		}
		return n, nil
	}, queue.WithRetry(5, gojob.NoBackoff()))

	var got []gojob.Result[int]
	for r := range results {
		got = append(got, r)
	}
	if len(got) != 1 || got[0].Err != nil {
		t.Fatalf("expected 1 success, got %+v", got)
	}
	if len(q.Dead()) != 0 {
		t.Error("expected nothing dead-lettered")
	}
}

func TestConsumeDeadLetter(t *testing.T) {
	ctx := context.Background()
	q := memq.New[int]()
	_ = q.Publish(ctx, 42)
	q.Seal()

	results, _ := queue.Consume(ctx, q, func(ctx context.Context, n int) (int, error) {
		return 0, fmt.Errorf("always fails")
	}, queue.WithRetry(1, gojob.NoBackoff()), queue.WithMaxDeliveries(3))

	var got []gojob.Result[int]
	for r := range results {
		got = append(got, r)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 finalized result, got %d", len(got))
	}
	if got[0].Err == nil {
		t.Error("expected an error result")
	}
	if dead := q.Dead(); len(dead) != 1 || dead[0] != 42 {
		t.Errorf("expected 42 dead-lettered, got %v", dead)
	}
}

// TestConsumeTimeoutKeepsHandle is the key regression test for the ack
// correlation: an attempt that times out must still carry its message handle so
// the message can be finalized (here, dead-lettered).
func TestConsumeTimeoutKeepsHandle(t *testing.T) {
	ctx := context.Background()
	q := memq.New[int]()
	_ = q.Publish(ctx, 1)
	q.Seal()

	results, _ := queue.Consume(ctx, q, func(ctx context.Context, n int) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(time.Second):
			return n, nil
		}
	}, queue.WithTimeout(20*time.Millisecond), queue.WithMaxDeliveries(1))

	var got []gojob.Result[int]
	for r := range results {
		got = append(got, r)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	if !errors.Is(got[0].Err, context.DeadlineExceeded) {
		t.Errorf("expected deadline exceeded, got %v", got[0].Err)
	}
	if len(q.Dead()) != 1 {
		t.Errorf("expected the message to be dead-lettered, dead=%d", len(q.Dead()))
	}
}

func TestFill(t *testing.T) {
	ctx := context.Background()
	q := memq.New[string]()
	in := make(chan string)
	go func() {
		defer close(in)
		in <- "a"
		in <- "b"
		in <- "c"
	}()
	n, err := queue.Fill(ctx, q, in)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Errorf("expected 3 published, got %d", n)
	}
}
