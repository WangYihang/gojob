package pipeline

import "context"

// Shard passes through only the items assigned to this shard, distributing by
// arrival order: the i-th item goes to shard i%numShards. It is the streaming
// analogue of running numShards copies of a job on different machines, each
// with a different shard index. A numShards of 1 (or less) returns in unchanged.
func Shard[T any](ctx context.Context, in <-chan T, numShards, shard int) <-chan T {
	if numShards <= 1 {
		return in
	}
	out := make(chan T)
	go func() {
		defer close(out)
		i := 0
		for v := range in {
			if i%numShards == shard {
				select {
				case out <- v:
				case <-ctx.Done():
					return
				}
			}
			i++
		}
	}()
	return out
}
