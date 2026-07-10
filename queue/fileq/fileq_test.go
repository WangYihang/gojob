package fileq_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/queue"
	"github.com/WangYihang/gojob/queue/fileq"
)

func TestRoundTrip(t *testing.T) {
	ctx := context.Background()
	q, err := fileq.Open[int](t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 8; i++ {
		if err := q.Publish(ctx, i); err != nil {
			t.Fatal(err)
		}
	}
	_ = q.Seal()

	results, err := queue.Consume(ctx, q, func(ctx context.Context, n int) (int, error) {
		return n, nil
	}, queue.WithWorkers(3))
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
	if len(got) != 8 {
		t.Errorf("expected 8 distinct results, got %d", len(got))
	}
	if q.DeadCount() != 0 {
		t.Errorf("expected 0 dead-lettered, got %d", q.DeadCount())
	}
}

func TestPersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	q1, _ := fileq.Open[string](dir)
	_ = q1.Publish(ctx, "hello")
	_ = q1.Publish(ctx, "world")
	_ = q1.Seal()
	_ = q1.Close()

	// A brand new queue over the same directory sees the persisted messages.
	q2, _ := fileq.Open[string](dir)
	results, _ := queue.Consume(ctx, q2, func(ctx context.Context, s string) (string, error) {
		return s, nil
	})
	got := map[string]bool{}
	for r := range results {
		got[r.Value] = true
	}
	if len(got) != 2 || !got["hello"] || !got["world"] {
		t.Errorf("expected {hello, world}, got %v", got)
	}
}

func TestDeadLetter(t *testing.T) {
	ctx := context.Background()
	q, _ := fileq.Open[int](t.TempDir(), fileq.WithLease(time.Second))
	_ = q.Publish(ctx, 5)
	_ = q.Seal()

	results, _ := queue.Consume(ctx, q, func(ctx context.Context, n int) (int, error) {
		return 0, fmt.Errorf("boom")
	}, queue.WithRetry(1, gojob.NoBackoff()), queue.WithMaxDeliveries(2))

	count := 0
	for r := range results {
		if r.Err == nil {
			t.Error("expected an error result")
		}
		count++
	}
	if count != 1 {
		t.Errorf("expected 1 finalized result, got %d", count)
	}
	if q.DeadCount() != 1 {
		t.Errorf("expected 1 dead-lettered, got %d", q.DeadCount())
	}
}

// TestCrashResume simulates a consumer that dies mid-job (its context is
// cancelled with messages still in flight) and verifies that a fresh consumer
// over the same directory reclaims the leased-but-unacked messages and finishes
// every job.
func TestCrashResume(t *testing.T) {
	dir := t.TempDir()
	const n = 12
	const lease = 150 * time.Millisecond

	prod, _ := fileq.Open[int](dir, fileq.WithLease(lease))
	for i := 0; i < n; i++ {
		if err := prod.Publish(context.Background(), i); err != nil {
			t.Fatal(err)
		}
	}
	_ = prod.Seal()

	var mu sync.Mutex
	seen := map[int]int{}
	record := func(v int) { mu.Lock(); seen[v]++; mu.Unlock() }
	distinct := func() int { mu.Lock(); defer mu.Unlock(); return len(seen) }

	// Phase 1: consumer that holds each message a while, then "crash" it.
	ctx1, cancel1 := context.WithCancel(context.Background())
	q1, _ := fileq.Open[int](dir, fileq.WithLease(lease))
	r1, _ := queue.Consume(ctx1, q1, func(ctx context.Context, v int) (int, error) {
		record(v)
		select {
		case <-ctx.Done():
		case <-time.After(60 * time.Millisecond):
		}
		return v, nil
	}, queue.WithWorkers(2))
	go func() {
		for range r1 {
		}
	}()

	deadline := time.Now().Add(3 * time.Second)
	for distinct() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	cancel1() // simulate kill -9

	// Phase 2: a fresh consumer resumes and drains everything.
	ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()
	q2, _ := fileq.Open[int](dir, fileq.WithLease(lease))
	r2, _ := queue.Consume(ctx2, q2, func(ctx context.Context, v int) (int, error) {
		record(v)
		return v, nil
	}, queue.WithWorkers(2), queue.WithMaxDeliveries(20))
	for range r2 {
	}

	if got := distinct(); got != n {
		t.Errorf("expected all %d jobs processed after resume, got %d distinct", n, got)
	}
}
