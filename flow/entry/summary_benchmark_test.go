package entry

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
)

// The flow-context wrapper around every request, with the [FLOW] summary line, at a level that drops it.
func BenchmarkFlowSummaryAtInfo(b *testing.B) {
	logger, _ := logging.NewLogger("INFO", io.Discard, "")
	var handler http.Handler
	run := withFlowContext(func(_ context.Context, _ config.ServiceConfig, h http.Handler) error {
		handler = h
		return nil
	}, logger)
	_ = run(context.Background(), config.ServiceConfig{}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	w := httptest.NewRecorder()
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		handler.ServeHTTP(w, req)
	}
}

type countingLogger struct {
	logging.Logger
	enabled bool
	debugs  int
}

func (l *countingLogger) Enabled(level int) bool { return l.enabled || level > logging.LEVEL_DEBUG }
func (l *countingLogger) Debug(...any)           { l.debugs++ }

// M2 2.4: below DEBUG the [FLOW] line is neither built nor passed to the logger.
func TestFlowSummaryOnlyAtDebug(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		logger := &countingLogger{Logger: logging.NoOp, enabled: enabled}
		var handler http.Handler
		_ = withFlowContext(func(_ context.Context, _ config.ServiceConfig, h http.Handler) error {
			handler = h
			return nil
		}, logger)(context.Background(), config.ServiceConfig{}, http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))

		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))

		if want := map[bool]int{false: 0, true: 1}[enabled]; logger.debugs != want {
			t.Errorf("debug enabled=%v: %d [FLOW] lines, want %d", enabled, logger.debugs, want)
		}
	}
}
