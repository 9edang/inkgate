package exporter

import (
	"github.com/9edang/inkgate/context"

	"github.com/9edang/inkgate/engine/logging"
	opencensus "github.com/9edang/inkgate/feature/telemetry/opencensus"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/trace"
)

func Register(l logging.Logger) {
	opencensus.RegisterExporterFactories(func(_ context.Context, _ opencensus.Config) (any, error) {
		return Logger{l}, nil
	})
}

type Logger struct {
	Logger logging.Logger
}

// ExportView logs the content of the received rows.
func (e Logger) ExportView(vd *view.Data) {
	if len(vd.Rows) == 0 {
		return
	}
	e.Logger.Debug(vd.View.Name, *vd)
	for _, row := range vd.Rows {
		e.Logger.Debug(vd.View.Name, row)
	}
}

// ExportView logs the content of the received span.
func (e Logger) ExportSpan(data *trace.SpanData) {
	if !data.IsSampled() {
		return
	}
	e.Logger.Debug(data.Name, *data)
}
