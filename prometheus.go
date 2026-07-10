package gojob

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/WangYihang/gojob/pkg/runner"
	"github.com/WangYihang/gojob/pkg/utils"
	"github.com/WangYihang/gojob/pkg/version"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/push"
	io_prometheus_client "github.com/prometheus/client_model/go"
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
		// Bind fresh variables so every LabelPair points at its own name/value
		// rather than at the shared range variables.
		name, value := k, v
		c.customLabels = append(c.customLabels, &io_prometheus_client.LabelPair{
			Name:  &name,
			Value: &value,
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
	// Gauges are created per pusher (rather than as package globals) so that
	// multiple schedulers in the same process do not share and clobber the same
	// metric values.
	numTotal := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "num_total",
		Help: "Total number of tasks",
	})
	numFailed := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "num_failed",
		Help: "Total number of failed tasks",
	})
	numSucceed := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "num_succeed",
		Help: "Total number of succeeded tasks",
	})
	numFinished := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "num_finished",
		Help: "Total number of finished tasks",
	})

	r := runner.Get()
	instance := fmt.Sprintf(
		"gojob-%s-%s-%s-%s",
		version.Version,
		utils.Sanitize(r.Country),
		utils.Sanitize(r.City),
		r.IP,
	)
	registry := NewRegistryWithLabels(map[string]string{
		"gojob_version":        version.Version,
		"gojob_runner_ip":      r.IP,
		"gojob_runner_country": r.Country,
		"gojob_runner_region":  r.Region,
		"gojob_runner_city":    r.City,
	})
	registry.MustRegister(numTotal, numFailed, numSucceed, numFinished)
	registry.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	go func() {
		defer wg.Done()
		for status := range statusChan {
			slog.Info("prometheus pusher", slog.Any("status", status))
			numTotal.Set(float64(status.NumTotal))
			numFailed.Set(float64(status.NumFailed))
			numSucceed.Set(float64(status.NumSucceed))
			numFinished.Set(float64(status.NumFinished))
			if err := push.New(url, job).Grouping(
				"instance", instance,
			).Gatherer(registry).Push(); err != nil {
				slog.Error("error occurred while pushing to prometheus", slog.String("error", err.Error()))
			}
		}
	}()
}
