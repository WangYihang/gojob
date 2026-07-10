package queue

import "context"

// Fill publishes every item read from in to q and returns the number published.
// It stops early if ctx is cancelled or a publish fails, returning the count so
// far along with the error. Use it to load a queue from any gojob source:
//
//	n, err := queue.Fill(ctx, q, gojob.Lines(ctx, "jobs.txt"))
func Fill[T any](ctx context.Context, q Queue[T], in <-chan T) (int, error) {
	n := 0
	for {
		select {
		case <-ctx.Done():
			return n, ctx.Err()
		case v, ok := <-in:
			if !ok {
				return n, nil
			}
			if err := q.Publish(ctx, v); err != nil {
				return n, err
			}
			n++
		}
	}
}
