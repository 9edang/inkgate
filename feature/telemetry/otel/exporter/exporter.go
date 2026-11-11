// Package exporter defines the interfaces required to implement in
// order to add additional exporters.
package exporter

import (
	"fmt"
	"github.com/9edang/inkgate/context"
	"maps"
	"sync"
	"time"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/9edang/inkgate/feature/telemetry/otel/config"
	"github.com/9edang/inkgate/feature/telemetry/otel/exporter/otelcollector"
	"github.com/9edang/inkgate/feature/telemetry/otel/exporter/prometheus"
)

// MetricReader is the interface required in order to
// export metrics.
type MetricReader interface {
	MetricReader(reportingPeriod time.Duration) sdkmetric.Reader
	MetricDefaultReporting() bool
}

// SpanExporter is the interface required in order to
// export traces.
type SpanExporter interface {
	SpanExporter() sdktrace.SpanExporter
	TraceDefaultReporting() bool
}

var (
	metricsInstances map[string]MetricReader
	tracesInstances  map[string]SpanExporter
	mu               = new(sync.RWMutex)
)

func CreateOTLPExporters(ctx context.Context, otlpConfs []config.OTLPExporter) (map[string]MetricReader, map[string]SpanExporter, error) {
	m := make(map[string]MetricReader, len(otlpConfs))
	s := make(map[string]SpanExporter, len(otlpConfs))
	for idx, ecfg := range otlpConfs {
		c, err := otelcollector.Exporter(ctx, ecfg)
		if err != nil {
			return nil, nil, fmt.Errorf("OTLP Exporter %s (at idx %d) failed: %s", ecfg.Name, idx, err.Error())
		}
		s[ecfg.Name] = c
		m[ecfg.Name] = c
	}
	return m, s, nil
}

func CreatePrometheusExporters(ctx context.Context, promConfs []config.PrometheusExporter) (map[string]MetricReader, error) {
	m := make(map[string]MetricReader, len(promConfs))
	for idx, ecfg := range promConfs {
		c, err := prometheus.Exporter(ctx, ecfg)
		if err != nil {
			return nil, fmt.Errorf("prometheus exporter %s (at idx %d) failed: %s", ecfg.Name, idx, err.Error())
		}
		m[ecfg.Name] = c
	}
	return m, nil
}

// Instances create instances for a given configuration.
func Instances(ctx context.Context, cfg *config.ConfigData) (map[string]MetricReader, map[string]SpanExporter, error) {
	// Create OTLP (OpenTelemetry Line Protocol) exporters
	m, s, err := CreateOTLPExporters(ctx, cfg.Exporters.OTLP)
	if err != nil {
		return nil, nil, err
	}
	// Create Prometheus exporters
	pm, err := CreatePrometheusExporters(ctx, cfg.Exporters.Prometheus)
	if err != nil {
		return nil, nil, err
	}
	maps.Copy(m, pm)
	return m, s, nil
}

// SetGlobalExporterInstances sets the provided metric and traces
// as global defaults.
func SetGlobalExporterInstances(m map[string]MetricReader, t map[string]SpanExporter) {
	mu.Lock()
	metricsInstances = make(map[string]MetricReader, len(m))
	tracesInstances = make(map[string]SpanExporter, len(t))
	maps.Copy(metricsInstances, m)
	maps.Copy(tracesInstances, t)
	mu.Unlock()
}

// GetGlobalExporterInstances gets the global metrics and traces exporters
func GetGlobalExporterInstances() (map[string]MetricReader, map[string]SpanExporter) {
	mu.RLock()
	m := make(map[string]MetricReader, len(metricsInstances))
	t := make(map[string]SpanExporter, len(tracesInstances))
	maps.Copy(m, metricsInstances)
	maps.Copy(t, tracesInstances)
	mu.RUnlock()
	return m, t
}

// GlobalTraceInstance gets a global trace exporter by name
func GlobalTraceInstance(name string) (SpanExporter, error) {
	mu.RLock()
	i, ok := tracesInstances[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return i, nil
}

// GlobalMetricInstance get a global metrics exporter by name
func GlobalMetricInstance(name string) (MetricReader, error) {
	mu.RLock()
	i, ok := metricsInstances[name]
	mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	return i, nil
}
