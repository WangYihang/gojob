package pipeline_test

import (
	"context"
	"testing"

	"github.com/WangYihang/gojob/pipeline"
)

func rangeInts(n int) []int {
	out := make([]int, n)
	for i := range out {
		out[i] = i
	}
	return out
}

func TestShardPartitions(t *testing.T) {
	ctx := context.Background()
	const numShards = 3
	seen := map[int]int{}
	for shard := 0; shard < numShards; shard++ {
		src := pipeline.From(ctx, rangeInts(15)...)
		for v := range pipeline.Shard(ctx, src, numShards, shard) {
			seen[v]++
			if v%numShards != shard {
				t.Errorf("shard %d received item %d (belongs to shard %d)", shard, v, v%numShards)
			}
		}
	}
	// Every item must have been handled by exactly one shard.
	for i := 0; i < 15; i++ {
		if seen[i] != 1 {
			t.Errorf("item %d handled %d times across shards, want 1", i, seen[i])
		}
	}
}

func TestShardSingle(t *testing.T) {
	ctx := context.Background()
	src := pipeline.From(ctx, 1, 2, 3)
	count := 0
	for range pipeline.Shard(ctx, src, 1, 0) {
		count++
	}
	if count != 3 {
		t.Errorf("numShards<=1 should pass everything through, got %d", count)
	}
}
