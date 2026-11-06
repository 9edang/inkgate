package zipkin

import (
	"errors"
	"net"

	"github.com/9edang/inkgate/context"

	"contrib.go.opencensus.io/exporter/zipkin"
	"github.com/openzipkin/zipkin-go/model"
	httpreporter "github.com/openzipkin/zipkin-go/reporter/http"

	opencensus "github.com/9edang/inkgate/feature/telemetry/opencensus"
)

func init() {
	opencensus.RegisterExporterFactories(func(ctx context.Context, cfg opencensus.Config) (any, error) {
		return Exporter(ctx, cfg)
	})
}

func Exporter(_ context.Context, cfg opencensus.Config) (*zipkin.Exporter, error) {
	if cfg.Exporters.Zipkin == nil {
		return nil, errDisabled
	}

	return zipkin.NewExporter(
		httpreporter.NewReporter(cfg.Exporters.Zipkin.CollectorURL),
		&model.Endpoint{
			ServiceName: cfg.Exporters.Zipkin.ServiceName,
			IPv4:        net.ParseIP(cfg.Exporters.Zipkin.IP),
			// #nosec G115 -- numeric value from the operator config, far below the overflow range
			Port: uint16(cfg.Exporters.Zipkin.Port),
		},
	), nil
}

var errDisabled = errors.New("opencensus zipkin exporter disabled")
