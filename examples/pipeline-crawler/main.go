// Command pipeline-crawler is the complex-http-crawler rewritten on top of the
// github.com/WangYihang/gojob/pipeline package, to show the composable design:
// a source, a concurrent Process stage with retries and timeouts, a Shard
// filter, decoupled progress via WithStats/ReportEvery, and a JSONL sink — all
// driven by a single cancellable context.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/WangYihang/gojob/pipeline"
	"github.com/WangYihang/uio"
)

// CrawlResult is the typed output of one crawl; it becomes Result.Value.
type CrawlResult struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
}

// crawl is an ordinary function — no interface to implement, trivially testable.
func crawl(ctx context.Context, url string) (CrawlResult, error) {
	result := CrawlResult{URL: url}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return result, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return result, err
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode
	return result, nil
}

func main() {
	var (
		input     = flag.String("i", "-", "input file: URLs one per line ('-' for stdin; gzip/S3 URLs via uio)")
		output    = flag.String("o", "-", "output file: JSONL results ('-' for stdout; gzip/S3 URLs via uio)")
		workers   = flag.Int("n", 32, "number of concurrent workers")
		retries   = flag.Int("r", 4, "max attempts per URL")
		timeout   = flag.Int("t", 16, "per-attempt timeout in seconds")
		numShards = flag.Int("s", 1, "total number of shards")
		shard     = flag.Int("d", 0, "index of this shard")
	)
	flag.Parse()

	// One context governs the whole pipeline; Ctrl-C cancels every stage.
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	out, err := uio.Open(*output)
	if err != nil {
		slog.Error("cannot open output", slog.String("path", *output), slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer out.Close()

	// source -> shard -> concurrent process (retry + timeout) -> stats -> sink
	urls := pipeline.Lines(ctx, *input)
	urls = pipeline.Shard(ctx, urls, *numShards, *shard)

	results := pipeline.Process(ctx, urls, crawl,
		pipeline.Workers(*workers),
		pipeline.Retry(*retries, pipeline.ExpBackoff(100*time.Millisecond, 10*time.Second)),
		pipeline.Timeout(time.Duration(*timeout)*time.Second),
	)

	// Observation is decoupled: progress taps the stream without altering it.
	// A Prometheus pusher would consume stats.Stream(...) the same way.
	results, stats := pipeline.WithStats(ctx, results)
	go pipeline.ReportEvery(stats, 5*time.Second, os.Stderr)

	if err := pipeline.WriteJSONL(ctx, out, results); err != nil {
		slog.Error("write failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
