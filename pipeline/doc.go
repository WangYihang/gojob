// Package pipeline models a resilient, observable, parallel job as a
// composition of small stages connected by channels.
//
// A source produces items, Process maps them concurrently with retries and
// per-attempt timeouts, combinators such as Shard and WithStats transform or
// observe the stream, and a sink such as WriteJSONL drives it to completion.
// Cancellation and shutdown flow from a single context.Context; "done" is
// simply a closed channel — there is no Start/Submit/Wait lifecycle and no
// shared mutable state to race on.
//
//	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
//	defer cancel()
//
//	urls := pipeline.Lines(ctx, "input.txt") // <-chan string
//	urls = pipeline.Shard(ctx, urls, 4, 0)   // keep this shard's slice
//
//	results := pipeline.Process(ctx, urls, crawl,
//		pipeline.Workers(32),
//		pipeline.Retry(4, pipeline.ExpBackoff(100*time.Millisecond, 10*time.Second)),
//		pipeline.Timeout(16*time.Second),
//	)
//
//	results, stats := pipeline.WithStats(ctx, results)
//	go pipeline.ReportEvery(stats, 5*time.Second, os.Stderr)
//
//	if err := pipeline.WriteJSONL(ctx, out, results); err != nil {
//		log.Fatal(err)
//	}
package pipeline
