package redisq_test

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/queue"
	"github.com/WangYihang/gojob/queue/redisq"
	"github.com/redis/go-redis/v9"
)

var sockCounter atomic.Int64

// startRedis launches a throwaway redis-server listening on a private unix
// socket (no TCP port, so no port-allocation races), skipping the test if the
// binary is not installed.
func startRedis(t *testing.T) *redis.Client {
	t.Helper()
	bin, err := exec.LookPath("redis-server")
	if err != nil {
		t.Skip("redis-server not installed")
	}
	sock := fmt.Sprintf("/tmp/gojob-redisq-%d-%d.sock", os.Getpid(), sockCounter.Add(1))
	t.Cleanup(func() { _ = os.Remove(sock) })

	cmd := exec.Command(bin, "--port", "0", "--unixsocket", sock,
		"--save", "", "--appendonly", "no", "--dir", t.TempDir())
	if err := cmd.Start(); err != nil {
		t.Fatalf("start redis-server: %v", err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	})

	rdb := redis.NewClient(&redis.Options{Network: "unix", Addr: sock})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for {
		if err := rdb.Ping(ctx).Err(); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal("redis-server did not become ready")
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Cleanup(func() { _ = rdb.Close() })
	return rdb
}

func TestRoundTrip(t *testing.T) {
	rdb := startRedis(t)
	ctx := context.Background()
	q := redisq.New[int](rdb, redisq.WithPollInterval(20*time.Millisecond))
	for i := 0; i < 10; i++ {
		if err := q.Publish(ctx, i); err != nil {
			t.Fatal(err)
		}
	}
	if err := q.Seal(ctx); err != nil {
		t.Fatal(err)
	}

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
		t.Errorf("expected 10 distinct results, got %d", len(got))
	}
	if q.DeadCount(ctx) != 0 {
		t.Errorf("expected 0 dead-lettered, got %d", q.DeadCount(ctx))
	}
}

func TestDeadLetter(t *testing.T) {
	rdb := startRedis(t)
	ctx := context.Background()
	q := redisq.New[int](rdb, redisq.WithPollInterval(20*time.Millisecond))
	_ = q.Publish(ctx, 5)
	_ = q.Seal(ctx)

	results, _ := queue.Consume(ctx, q, func(ctx context.Context, n int) (int, error) {
		return 0, fmt.Errorf("boom")
	}, queue.WithRetry(1, gojob.NoBackoff()), queue.WithMaxDeliveries(3))

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
	if q.DeadCount(ctx) != 1 {
		t.Errorf("expected 1 dead-lettered, got %d", q.DeadCount(ctx))
	}
}

// TestCrashResume cancels a consumer with messages in flight and verifies a
// fresh consumer over the same Redis queue reclaims the leased messages after
// the lease expires and finishes every job.
func TestCrashResume(t *testing.T) {
	rdb := startRedis(t)
	const n = 12
	const lease = 200 * time.Millisecond

	prod := redisq.New[int](rdb)
	for i := 0; i < n; i++ {
		if err := prod.Publish(context.Background(), i); err != nil {
			t.Fatal(err)
		}
	}
	_ = prod.Seal(context.Background())

	var mu sync.Mutex
	seen := map[int]int{}
	record := func(v int) { mu.Lock(); seen[v]++; mu.Unlock() }
	distinct := func() int { mu.Lock(); defer mu.Unlock(); return len(seen) }

	ctx1, cancel1 := context.WithCancel(context.Background())
	q1 := redisq.New[int](rdb, redisq.WithLease(lease), redisq.WithPollInterval(20*time.Millisecond))
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
	cancel1() // simulate crash

	ctx2, cancel2 := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel2()
	q2 := redisq.New[int](rdb, redisq.WithLease(lease), redisq.WithPollInterval(20*time.Millisecond))
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
