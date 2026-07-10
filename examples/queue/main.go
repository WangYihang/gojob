// Command queue demonstrates the durable, resumable file-backed queue.
//
//	go run ./examples/queue -fill -n 200      # publish 200 jobs and seal
//	go run ./examples/queue                    # consume until drained
//
// Kill the consumer mid-run (Ctrl-C or kill -9) and start it again: it resumes
// the un-acknowledged jobs. Run several consumers at once to share the work.
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
)

func main() {
	var (
		dir     = flag.String("dir", "jobs.q", "queue directory")
		fill    = flag.Bool("fill", false, "publish jobs and seal the queue, then exit")
		n       = flag.Int("n", 200, "number of jobs to publish (with -fill)")
		workers = flag.Int("w", 4, "number of consumer workers")
	)
	flag.Parse()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	q, err := fileq.Open[int](*dir, fileq.WithLease(5*time.Second))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if *fill {
		for i := 0; i < *n; i++ {
			if err := q.Publish(ctx, i); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		if err := q.Seal(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "published %d jobs to %s\n", *n, *dir)
		return
	}

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
