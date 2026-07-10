package gojob_test

import (
	"context"
	"testing"

	"github.com/WangYihang/gojob"
)

func TestShardPartitions(t *testing.T) {
	ctx := context.Background()
	const numShards = 3
	seen := map[int]int{}
	for shard := 0; shard < numShards; shard++ {
		src := gojob.From(ctx, rangeInts(15)...)
		for v := range gojob.Shard(ctx, src, numShards, shard) {
			seen[v]++
			if v%numShards != shard {
				t.Errorf("shard %d received item %d (belongs to shard %d)", shard, v, v%numShards)
			}
		}
	}
	for i := 0; i < 15; i++ {
		if seen[i] != 1 {
			t.Errorf("item %d handled %d times across shards, want 1", i, seen[i])
		}
	}
}

func TestShardSingle(t *testing.T) {
	ctx := context.Background()
	src := gojob.From(ctx, 1, 2, 3)
	count := 0
	for range gojob.Shard(ctx, src, 1, 0) {
		count++
	}
	if count != 3 {
		t.Errorf("numShards<=1 should pass everything through, got %d", count)
	}
}
