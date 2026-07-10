package pipeline_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/WangYihang/gojob/pipeline"
)

func TestWithStats(t *testing.T) {
	ctx := context.Background()
	src := pipeline.From(ctx, 1, 2, 3, 4, 5)
	results := pipeline.Process(ctx, src, func(ctx context.Context, n int) (int, error) {
		if n%2 == 0 {
			return 0, fmt.Errorf("even")
		}
		return n, nil
	})
	results, stats := pipeline.WithStats(ctx, results, pipeline.Total(5))
	for range results {
	}
	snap := stats.Snapshot()
	if snap.Total != 5 || snap.Done != 5 || snap.Failed != 2 || snap.Succeeded != 3 {
		t.Errorf("unexpected snapshot: %+v", snap)
	}
}

func TestStatsStreamFinalizes(t *testing.T) {
	ctx := context.Background()
	src := pipeline.From(ctx, 1, 2, 3)
	results := pipeline.Process(ctx, src, func(ctx context.Context, n int) (int, error) { return n, nil })
	results, stats := pipeline.WithStats(ctx, results)
	go func() {
		for range results {
		}
	}()
	var last pipeline.Snapshot
	for snap := range stats.Stream(1 * time.Millisecond) {
		last = snap
	}
	if last.Done != 3 {
		t.Errorf("expected final snapshot Done=3, got %d", last.Done)
	}
}
