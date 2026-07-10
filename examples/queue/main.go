// Command queue demonstrates the durable, resumable queue over either backend.
//
//	go run ./examples/queue -fill -n 200                 # file backend: publish + seal
//	go run ./examples/queue                               # file backend: consume until drained
//	go run ./examples/queue -redis localhost:6379 -fill   # redis backend
//	go run ./examples/queue -redis localhost:6379         # redis backend, run several at once
//
// Kill a consumer mid-run (Ctrl-C or kill -9) and start it again: it resumes the
// un-acknowledged jobs. The consume code is identical for both backends.
package main

import (
	"context"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"time"

	"github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/queue"
	"github.com/WangYihang/gojob/queue/fileq"
	"github.com/WangYihang/gojob/queue/redisq"
	"github.com/redis/go-redis/v9"
)

func main() {
	var (
		dir       = flag.String("dir", "jobs.q", "queue directory (file backend)")
		redisAddr = flag.String("redis", "", "redis address (uses the redis backend when set)")
		fill      = flag.Bool("fill", false, "publish jobs and seal the queue, then exit")
		n         = flag.Int("n", 200, "number of jobs to publish (with -fill)")
		workers   = flag.Int("w", 4, "number of consumer workers")
	)
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	var (
		q    queue.Queue[int]
		seal func() error
	)
	if *redisAddr != "" {
		rdb := redis.NewClient(&redis.Options{Addr: *redisAddr})
		defer rdb.Close()
		rq := redisq.New[int](rdb, redisq.WithLease(5*time.Second))
		q, seal = rq, func() error { return rq.Seal(ctx) }
	} else {
		fq, err := fileq.Open[int](*dir, fileq.WithLease(5*time.Second))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		q, seal = fq, fq.Seal
	}

	if *fill {
		for i := 0; i < *n; i++ {
			if err := q.Publish(ctx, i); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		if err := seal(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "published %d jobs\n", *n)
		return
	}

	// The same pipeline drives either backend.
	results, err := queue.Consume(ctx, q, func(ctx context.Context, i int) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(time.Duration(20+rand.Intn(60)) * time.Millisecond):
		}
		return i * i, nil
	},
		queue.WithWorkers(*workers),
		queue.WithRetry(3, gojob.ExpBackoff(100*time.Millisecond, time.Second)),
		queue.WithTimeout(10*time.Second),
		queue.WithMaxDeliveries(5),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	results, stats := gojob.WithStats(ctx, results)
	go gojob.ReportEvery(stats, time.Second, os.Stderr)
	if err := gojob.WriteJSONL(ctx, os.Stdout, results); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
	fmt.Fprintln(os.Stderr, "consumer drained")
}
