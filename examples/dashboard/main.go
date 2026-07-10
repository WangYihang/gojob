// Command dashboard runs a synthetic job and serves a live progress dashboard
// at http://localhost:8080 via the decoupled gojob/web package.
package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/web"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	const n = 500
	items := make([]int, n)
	for i := range items {
		items[i] = i
	}

	results := gojob.Process(ctx, gojob.From(ctx, items...), func(ctx context.Context, i int) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(time.Duration(20+rand.Intn(80)) * time.Millisecond):
		}
		if rand.Intn(12) == 0 {
			return i, fmt.Errorf("simulated failure")
		}
		return i, nil
	}, gojob.WithWorkers(8), gojob.WithRetry(2, gojob.NoBackoff()))

	results, stats := gojob.WithStats(ctx, results, gojob.WithTotal(n))
	go web.Serve(ctx, stats, ":8080", web.WithTitle("demo job"))
	fmt.Fprintln(os.Stderr, "watch live progress at http://localhost:8080")

	gojob.Drain(results)
	fmt.Fprintln(os.Stderr, "job complete")
}
