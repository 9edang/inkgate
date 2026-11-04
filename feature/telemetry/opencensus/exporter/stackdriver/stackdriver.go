package stackdriver

import (
	"errors"
	"github.com/9edang/inkgate/context"
	"time"

	"contrib.go.opencensus.io/exporter/stackdriver"
	"contrib.go.opencensus.io/exporter/stackdriver/monitoredresource"
	opencensus "github.com/9edang/inkgate/feature/telemetry/opencensus"
)

func init() {
	opencensus.RegisterExporterFactories(func(ctx context.Context, cfg opencensus.Config) (any, error) {
		return Exporter(ctx, cfg)
	})
}

var defaultMetricPrefix = "inkgate"

func Exporter(_ context.Context, cfg opencensus.Config) (*stackdriver.Exporter, error) {
	if cfg.Exporters.Stackdriver == nil {
		return nil, errors.New("stackdriver exporter disabled")
	}
	if cfg.Exporters.Stackdriver.MetricPrefix == "" {
		cfg.Exporters.Stackdriver.MetricPrefix = defaultMetricPrefix
	}

	labels := &stackdriver.Labels{}
	for k, v := range cfg.Exporters.Stackdriver.DefaultLabels {
		labels.Set(k, v, "")
	}

	return stackdriver.NewExporter(stackdriver.Options{
		ProjectID:               cfg.Exporters.Stackdriver.ProjectID,
		MetricPrefix:            cfg.Exporters.Stackdriver.MetricPrefix,
		BundleDelayThreshold:    time.Duration(cfg.ReportingPeriod) * time.Second,
		BundleCountThreshold:    cfg.SampleRate,
		DefaultMonitoringLabels: labels,
		MonitoredResource:       monitoredresource.Autodetect(),
	})
}
