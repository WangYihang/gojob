// Command prometheus shows observability as a plain Stats consumer: the same
// pipeline as the crawler, with progress pushed to a Prometheus Pushgateway via
// the decoupled gojob/prom package (no coupling in the core library).
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/WangYihang/gojob"
	"github.com/WangYihang/gojob/prom"
)

type CrawlResult struct {
	URL        string `json:"url"`
	StatusCode int    `json:"status_code"`
}

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
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	urls := make([]string, 0, 256)
	for i := 0; i < 256; i++ {
		urls = append(urls, fmt.Sprintf("https://httpbin.org/status/200?i=%d", i))
	}

	results := gojob.Process(ctx, gojob.From(ctx, urls...), crawl,
		gojob.WithWorkers(8),
		gojob.WithRetry(4, gojob.ExpBackoff(100*time.Millisecond, 10*time.Second)),
		gojob.WithTimeout(16*time.Second),
	)
	results, stats := gojob.WithStats(ctx, results, gojob.WithTotal(int64(len(urls))))

	// Observability is just a Stats consumer, wired up by the caller.
	go prom.Push(ctx, stats, "http://localhost:9091", "gojob", prom.WithInterval(5*time.Second))

	_ = gojob.WriteJSONL(ctx, os.Stdout, results)
}
