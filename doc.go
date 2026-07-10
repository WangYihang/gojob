// Package gojob models a resilient, observable, parallel job as a composition
// of small stages connected by channels.
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
//	urls := gojob.Lines(ctx, "input.txt") // <-chan string
//	urls = gojob.Shard(ctx, urls, 4, 0)   // keep this shard's slice
//
//	results := gojob.Process(ctx, urls, crawl,
//		gojob.WithWorkers(32),
//		gojob.WithRetry(4, gojob.ExpBackoff(100*time.Millisecond, 10*time.Second)),
//		gojob.WithTimeout(16*time.Second),
//	)
//
//	results, stats := gojob.WithStats(ctx, results)
//	go gojob.ReportEvery(stats, 5*time.Second, os.Stderr)
//
//	if err := gojob.WriteJSONL(ctx, out, results); err != nil {
//		log.Fatal(err)
//	}
package gojob
