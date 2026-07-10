# Go(od) Job

[![Go Reference](https://pkg.go.dev/badge/github.com/WangYihang/gojob.svg)](https://pkg.go.dev/github.com/WangYihang/gojob)
[![Go Report Card](https://goreportcard.com/badge/github.com/WangYihang/gojob)](https://goreportcard.com/report/github.com/WangYihang/gojob)
[![codecov](https://codecov.io/gh/WangYihang/gojob/graph/badge.svg?token=FG1HT7FCKG)](https://codecov.io/gh/WangYihang/gojob)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FWangYihang%2Fgojob.svg?type=shield)](https://app.fossa.com/projects/git%2Bgithub.com%2FWangYihang%2Fgojob?ref=badge_shield)

gojob runs a large batch of jobs concurrently — with retries, per-attempt
timeouts, sharding, live progress, and pluggable inputs/outputs — modeled as a
small, composable, cancellable **pipeline**.

A job is a composition of stages connected by channels: a **source** produces
items, **`Process`** maps them concurrently, **combinators** like `Shard` and
`WithStats` transform or observe the stream, and a **sink** such as `WriteJSONL`
drives it to completion. Everything is governed by a single `context.Context`;
"done" is just a closed channel — there is no `Start`/`Submit`/`Wait` lifecycle
and no shared mutable state to race on.

## Install

```
go get github.com/WangYihang/gojob
```

## Quick start

Crawl a list of URLs concurrently and write one JSON result per line. The unit
of work is an ordinary function — no interface to implement:

```go
package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/WangYihang/gojob"
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

	urls := gojob.Lines(ctx, "urls.txt") // <-chan string ("-" = stdin; gzip/S3 via uio)
	urls = gojob.Shard(ctx, urls, 1, 0)  // 1 shard = everything

	results := gojob.Process(ctx, urls, crawl,
		gojob.WithWorkers(32),
		gojob.WithRetry(4, gojob.ExpBackoff(100*time.Millisecond, 10*time.Second)),
		gojob.WithTimeout(16*time.Second),
	)

	results, stats := gojob.WithStats(ctx, results)
	go gojob.ReportEvery(stats, 5*time.Second, os.Stderr) // live progress, decoupled

	_ = gojob.WriteJSONL(ctx, os.Stdout, results)
}
```

Each output line is a `Result` envelope:

```json
{"value":{"url":"https://example.com/","status_code":200},"error":"","attempts":1,"started_at":1708934911909748,"duration_ms":42}
```

## Concepts

| Stage | What it does |
| --- | --- |
| `Lines(ctx, path)` / `From(ctx, items...)` | **Sources** — stream items from a file/stdin/gzip/S3 (via [uio](https://github.com/WangYihang/uio)) or from memory. |
| `Process(ctx, in, fn, opts...)` | The **engine** — runs `fn` over the stream with a bounded worker pool, returning `<-chan Result[Out]` in completion order. |
| `Shard(ctx, in, n, i)` | Keep only this shard's slice of the stream (item `k` → shard `k % n`). |
| `WithStats(ctx, in)` → `stats` | Tap the stream to count progress **without altering it**. |
| `ReportEvery` / `Stats.Stream` / `Stats.Snapshot` | Observe progress (stderr line, snapshot channel, one-off snapshot). |
| `WriteJSONL(ctx, w, in)` / `Tee` / `Drain` | **Sinks** — write JSON Lines to any `io.Writer`, fan a stream out, or discard it. |

### Retries and timeouts

`WithRetry` and `WithTimeout` are `Process` options. `WithRetry(n, backoff)` attempts each
item up to `n` times; `WithTimeout(d)` bounds a single attempt (its context is
cancelled when it elapses, so context-aware work aborts instead of leaking).
`Result` carries `Attempts`, `StartedAt`, and `Duration`.

```go
results := gojob.Process(ctx, in, fn,
	gojob.WithWorkers(64),
	gojob.WithRetry(5, gojob.ExpBackoff(100*time.Millisecond, 10*time.Second)),
	gojob.WithTimeout(30*time.Second),
)
```

### Sharding across machines

Run the same program on N machines, each with a different shard index, to split
the work deterministically:

```go
in = gojob.Shard(ctx, in, numShards, shard) // e.g. Shard(ctx, in, 4, 2)
```

### Self-contained tasks

Prefer "one task object per item"? Implement `Task[T]` and use `Execute`, which
is a thin adapter over `Process`:

```go
type PingTask struct {
	Host    string `json:"host"`
	Latency int64  `json:"latency_ms"`
}

func (t *PingTask) Execute(ctx context.Context) (*PingTask, error) { /* ... */ return t, nil }

results := gojob.Execute(ctx, tasks, gojob.WithWorkers(8)) // tasks is <-chan gojob.Task[*PingTask]
```

## Prometheus

Observability is just a `Stats` consumer, so it lives in a separate package and
the core library stays dependency-light. Import `gojob/prom` to push progress to
a Pushgateway:

```go
results, stats := gojob.WithStats(ctx, results)
go prom.Push(ctx, stats, "http://localhost:9091", "gojob") // gojob_num_total, _done, _succeeded, _failed
```

A `docker-compose.yaml` with Prometheus, a Pushgateway, and Grafana is included
for local experimentation.

## Web dashboard

Also a `Stats` consumer: `gojob/web` serves a small, self-contained live
dashboard (Server-Sent Events + one theme-adaptive HTML page, no external
assets). Point a browser at it to watch progress in real time:

```go
results, stats := gojob.WithStats(ctx, results)
go web.Serve(ctx, stats, ":8080", web.WithTitle("my job"))
```

Mount it on your own router with `web.Handler(ctx, stats)` instead. See
[examples/dashboard](./examples/dashboard/).

## Durable queue

For jobs that must **survive crashes** or be **shared across processes**, pull
work from a durable queue instead of an in-memory source. `queue.Consume`
returns the same `Result` stream as `Process`, so the rest of the pipeline is
unchanged; delivery is at-least-once (make `fn` idempotent).

```go
q, _ := fileq.Open[Job]("jobs.q")           // file-backed, crash-resumable
results, _ := queue.Consume(ctx, q, handle,
    queue.WithWorkers(32),
    queue.WithRetry(4, gojob.ExpBackoff(100*time.Millisecond, 10*time.Second)),
    queue.WithTimeout(16*time.Second),
    queue.WithMaxDeliveries(5),             // dead-letter after 5 tries
)
results, stats := gojob.WithStats(ctx, results)
gojob.WriteJSONL(ctx, out, results)
```

Backends: `queue/memq` (in-memory, stdlib) and `queue/fileq` (durable, stdlib,
resumes un-acknowledged work after a crash). Design notes and the planned Redis
backend are in [docs/design/queue.md](./docs/design/queue.md).

## Examples

| Example | Shows |
| --- | --- |
| [`examples/crawler`](./examples/crawler/) | Full CLI: file/stdin/S3 in, JSONL out, retries, timeouts, sharding, progress. |
| [`examples/stdio`](./examples/stdio/) | The smallest possible pipeline (stdin → transform → stdout). |
| [`examples/tasks`](./examples/tasks/) | The `Execute` + `Task[T]` object style. |
| [`examples/prometheus`](./examples/prometheus/) | Progress pushed via `gojob/prom`. |
| [`examples/dashboard`](./examples/dashboard/) | Live web dashboard via `gojob/web`. |
| [`examples/queue`](./examples/queue/) | Durable, crash-resumable jobs via `gojob/queue`. |

```bash
go run ./examples/crawler -i urls.txt -o results.jsonl -n 32 -r 4 -t 16
```

## Testing

Tests are layered; use the `Makefile`:

```bash
make test              # unit tests (race detector)
make test-integration  # + integration tests against a local HTTP server
make test-e2e          # build the example binary and drive it end-to-end
make cover             # coverage profile (unit + integration)
```

## License
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2FWangYihang%2Fgojob.svg?type=large)](https://app.fossa.com/projects/git%2Bgithub.com%2FWangYihang%2Fgojob?ref=badge_large)
