// Package prom pushes gojob progress to a Prometheus Pushgateway. It is a plain
// consumer of a *gojob.Stats handle, so observability stays fully decoupled from
// the pipeline: importing gojob does not pull in Prometheus; only importing this
// package does.
package prom

import (
	"context"
	"log/slog"
	"time"

	"github.com/WangYihang/gojob"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

type config struct {
	interval time.Duration
	labels   map[string]string
}

// Option configures Push.
type Option func(*config)

// WithInterval sets how often snapshots are pushed (default 5s).
func WithInterval(d time.Duration) Option {
	return func(c *config) {
		if d > 0 {
			c.interval = d
		}
	}
}

// WithLabel adds a grouping label (e.g. an instance or version) to every push.
func WithLabel(key, value string) Option {
	return func(c *config) {
		c.labels[key] = value
	}
}

// Push streams snapshots from stats to a Prometheus Pushgateway at url under the
// given job, until the observed job completes or ctx is cancelled. It blocks, so
// run it in its own goroutine:
//
//	results, stats := gojob.WithStats(ctx, results)
//	go prom.Push(ctx, stats, "http://localhost:9091", "gojob")
func Push(ctx context.Context, stats *gojob.Stats, url, job string, opts ...Option) error {
	cfg := config{interval: 5 * time.Second, labels: map[string]string{}}
	for _, o := range opts {
		o(&cfg)
	}

	var (
		total     = gauge("gojob_num_total", "Total number of tasks")
		done      = gauge("gojob_num_done", "Number of finished tasks")
		succeeded = gauge("gojob_num_succeeded", "Number of succeeded tasks")
		failed    = gauge("gojob_num_failed", "Number of failed tasks")
	)
	registry := prometheus.NewRegistry()
	registry.MustRegister(total, done, succeeded, failed)

	pusher := push.New(url, job).Gatherer(registry)
	for k, v := range cfg.labels {
		pusher = pusher.Grouping(k, v)
	}

	snapshots := stats.Stream(cfg.interval)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case snap, ok := <-snapshots:
			if !ok {
				return nil
			}
			total.Set(float64(snap.Total))
			done.Set(float64(snap.Done))
			succeeded.Set(float64(snap.Succeeded))
			failed.Set(float64(snap.Failed))
			if err := pusher.Push(); err != nil {
				slog.Error("gojob/prom: push failed", slog.String("error", err.Error()))
			}
		}
	}
}

func gauge(name, help string) prometheus.Gauge {
	return prometheus.NewGauge(prometheus.GaugeOpts{Name: name, Help: help})
}
