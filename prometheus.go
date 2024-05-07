package gojob

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/WangYihang/gojob/pkg/runner"
	"github.com/WangYihang/gojob/pkg/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/push"
	io_prometheus_client "github.com/prometheus/client_model/go"
)

var (
	numTotal = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "num_total",
			Help: "Total number of processed events",
		},
	)
	numFailed = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "num_failed",
			Help: "Total number of failed events",
		},
	)
	numSucceed = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "num_succeed",
			Help: "Total number of succeeded events",
		},
	)
	numFinished = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "num_finished",
			Help: "Total number of finished events",
		},
	)
)

type customMetricsRegistry struct {
	*prometheus.Registry
	customLabels []*io_prometheus_client.LabelPair
}

func NewRegistryWithLabels(labels map[string]string) *customMetricsRegistry {
	c := &customMetricsRegistry{
		Registry: prometheus.NewRegistry(),
	}
	for k, v := range labels {
		c.customLabels = append(c.customLabels, &io_prometheus_client.LabelPair{
			Name:  &k,
			Value: &v,
		})
	}
	return c
}

func (g *customMetricsRegistry) Gather() ([]*io_prometheus_client.MetricFamily, error) {
	metricFamilies, err := g.Registry.Gather()

	for _, metricFamily := range metricFamilies {
		metrics := metricFamily.Metric
		for _, metric := range metrics {
			metric.Label = append(metric.Label, g.customLabels...)
		}
	}

	return metricFamilies, err
}
func prometheusPusher(url, job string, statusChan <-chan Status, wg *sync.WaitGroup) {
	instance := fmt.Sprintf(
		"gojob-%s-%s-%s",
		version.Version,
		strings.ToLower(runner.Runner.Country),
		runner.Runner.IP,
	)
	registry := NewRegistryWithLabels(map[string]string{
		"gojob_version":        version.Version,
		"gojob_runner_ip":      runner.Runner.IP,
		"gojob_runner_country": runner.Runner.Country,
		"gojob_runner_region":  runner.Runner.Region,
		"gojob_runner_city":    runner.Runner.City,
		"gojob_instance":       instance,
	})
	registry.MustRegister(numTotal, numFailed, numSucceed, numFinished)
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	go func() {
		for status := range statusChan {
			slog.Info("promehteus pusher", slog.Any("status", status))
			numTotal.Set(float64(status.NumTotal))
			numFailed.Set(float64(status.NumFailed))
			numSucceed.Set(float64(status.NumSucceed))
			numFinished.Set(float64(status.NumFinished))
			if err := push.New(url, job).Gatherer(registry).Push(); err != nil {
				slog.Error("error occurred while pushing to prometheus", slog.String("error", err.Error()))
			}
		}
		wg.Done()
	}()
}
