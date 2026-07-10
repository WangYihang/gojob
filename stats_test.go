package gojob_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/WangYihang/gojob"
)

func TestWithStats(t *testing.T) {
	ctx := context.Background()
	src := gojob.From(ctx, 1, 2, 3, 4, 5)
	results := gojob.Process(ctx, src, func(ctx context.Context, n int) (int, error) {
		if n%2 == 0 {
			return 0, fmt.Errorf("even")
		}
		return n, nil
	})
	results, stats := gojob.WithStats(ctx, results, gojob.WithTotal(5))
	for range results {
	}
	snap := stats.Snapshot()
	if snap.Total != 5 || snap.Done != 5 || snap.Failed != 2 || snap.Succeeded != 3 {
		t.Errorf("unexpected snapshot: %+v", snap)
	}
}

func TestStatsUnknownTotal(t *testing.T) {
	ctx := context.Background()
	src := gojob.From(ctx, 1, 2, 3)
	results := gojob.Process(ctx, src, func(ctx context.Context, n int) (int, error) { return n, nil })
	results, stats := gojob.WithStats(ctx, results)
	for range results {
	}
	if snap := stats.Snapshot(); snap.Total != -1 {
		t.Errorf("expected unknown total (-1), got %d", snap.Total)
	}
}

func TestStatsDone(t *testing.T) {
	ctx := context.Background()
	src := gojob.From(ctx, 1, 2, 3)
	results := gojob.Process(ctx, src, func(ctx context.Context, n int) (int, error) { return n, nil })
	results, stats := gojob.WithStats(ctx, results)
	go func() {
		for range results {
		}
	}()
	select {
	case <-stats.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("stats.Done() never closed")
	}
	if snap := stats.Snapshot(); snap.Done != 3 {
		t.Errorf("expected Done=3 after completion, got %d", snap.Done)
	}
}

func TestStatsStreamFinalizes(t *testing.T) {
	ctx := context.Background()
	src := gojob.From(ctx, 1, 2, 3)
	results := gojob.Process(ctx, src, func(ctx context.Context, n int) (int, error) { return n, nil })
	results, stats := gojob.WithStats(ctx, results)
	go func() {
		for range results {
		}
	}()
	var last gojob.Snapshot
	for snap := range stats.Stream(1 * time.Millisecond) {
		last = snap
	}
	if last.Done != 3 {
		t.Errorf("expected final snapshot Done=3, got %d", last.Done)
	}
}
