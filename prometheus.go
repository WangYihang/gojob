package gojob

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/WangYihang/gojob/pkg/runner"
	"github.com/WangYihang/gojob/pkg/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
)

var (
	numTotal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "num_total",
			Help: "Total number of processed events",
		},
		[]string{"version", "runner_ip", "runner_country", "runner_region", "runner_city"},
	)
	numFailed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "num_failed",
			Help: "Total number of failed events",
		},
		[]string{"version", "runner_ip", "runner_country", "runner_region", "runner_city"},
	)
	numSucceed = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "num_succeed",
			Help: "Total number of succeeded events",
		},
		[]string{"version", "runner_ip", "runner_country", "runner_region", "runner_city"},
	)
	numFinished = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "num_finished",
			Help: "Total number of finished events",
		},
		[]string{"version", "runner_ip", "runner_country", "runner_region", "runner_city"},
	)
)

func prometheusPusher(url, job string, statusChan <-chan Status, wg *sync.WaitGroup) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(numTotal, numFailed, numSucceed, numFinished)
	instance := fmt.Sprintf(
		"gojob-%s-%s-%s",
		version.Version,
		strings.ToLower(runner.Runner.Country),
		runner.Runner.IP,
	)
	labels := prometheus.Labels{
		"version":        version.Version,
		"runner_ip":      runner.Runner.IP,
		"runner_country": runner.Runner.Country,
		"runner_region":  runner.Runner.Region,
		"runner_city":    runner.Runner.City,
	}
	go func() {
		for status := range statusChan {
			slog.Info("promehteus pusher", slog.Any("status", status))
			numTotal.With(labels).Set(float64(status.NumTotal))
			numFailed.With(labels).Set(float64(status.NumFailed))
			numSucceed.With(labels).Set(float64(status.NumSucceed))
			numFinished.With(labels).Set(float64(status.NumFinished))
			if err := push.New(url, job).Grouping(
				"instance", instance,
			).Gatherer(registry).Push(); err != nil {
				slog.Error("error occurred while pushing to prometheus", slog.String("error", err.Error()))
			}
		}
		wg.Done()
	}()
}
