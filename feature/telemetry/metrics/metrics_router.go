package metrics

import (
	"bufio"
	"errors"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/9edang/inkgate/context"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/router"
)

// NewHTTPMetrics creates a new metrics producer that also serves the stats endpoint and instruments the router.
func NewHTTPMetrics(ctx context.Context, e config.ExtraConfig, l logging.Logger) *HTTPMetrics {
	metricsCollector := HTTPMetrics{New(ctx, e, l)}
	if metricsCollector.Config != nil && !metricsCollector.Config.EndpointDisabled {
		metricsCollector.RunEndpoint(ctx, metricsCollector.NewEngine(), l)
	}

	return &metricsCollector
}

// HTTPMetrics is the component that manages all the metrics of the HTTP router.
type HTTPMetrics struct {
	*Metrics
}

// RunEndpoint serves the stats endpoint with the logger.
func (m *HTTPMetrics) RunEndpoint(ctx context.Context, s *http.ServeMux, l logging.Logger) {
	server := &http.Server{
		Addr:              m.Config.ListenAddr,
		Handler:           s,
		ReadHeaderTimeout: 3 * time.Second,
	}
	go func() {
		l.Error(server.ListenAndServe())
	}()

	go func() {
		<-ctx.Done()
		l.Info("shutting down the stats handler")
		ctx, cancel := context.WithTimeout(ctx, time.Second)
		// #nosec G104 -- best-effort shutdown, the error is not actionable
		server.Shutdown(ctx)
		cancel()
	}()
}

// NewEngine returns a *http.ServeMux with the stats endpoint (no logger).
func (m *HTTPMetrics) NewEngine() *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("/__stats", m.NewExpHandler())

	return mux
}

// NewExpHandler creates an http.Handler ready to expose all the collected metrics as a JSON.
func (m *HTTPMetrics) NewExpHandler() http.Handler {
	return NewExpHandler(m.Registry)
}

// NewHTTPHandler wraps an http.Handler adding some simple instrumentation to the handler.
func (m *HTTPMetrics) NewHTTPHandler(name string, h http.Handler) http.HandlerFunc {
	return NewHTTPHandler(name, h, m.Router)
}

func (m *HTTPMetrics) NewHTTPHandlerFactory(defaultHandlerFactory router.HandlerFactory) router.HandlerFactory {
	if m.Config == nil || m.Config.RouterDisabled {
		return defaultHandlerFactory
	}

	return func(cfg *config.EndpointConfig, p proxy.Proxy) http.HandlerFunc {
		return m.NewHTTPHandler(cfg.Endpoint, defaultHandlerFactory(cfg, p))
	}
}

// NewHTTPHandler wraps an http.Handler adding some simple instrumentation to the handler.
func NewHTTPHandler(name string, h http.Handler, rm *RouterMetrics) http.HandlerFunc {
	rm.RegisterResponseWriterMetrics(name)
	return func(w http.ResponseWriter, r *http.Request) {
		rm.Connection(r.TLS)
		rw := newHTTPResponseWriter(name, w, rm)
		h.ServeHTTP(rw, r)
		rw.end()
		rm.Disconnection()
	}
}

func newHTTPResponseWriter(name string, rw http.ResponseWriter, rm *RouterMetrics) *responseWriter {
	return &responseWriter{
		ResponseWriter: rw,
		begin:          time.Now(),
		name:           name,
		rm:             rm,
		status:         200,
	}
}

type responseWriter struct {
	http.ResponseWriter
	begin        time.Time
	name         string
	rm           *RouterMetrics
	responseSize int
	status       int
}

// WriteHeader implements the http.ResponseWriter interface.
func (w *responseWriter) WriteHeader(code int) {
	w.ResponseWriter.WriteHeader(code)
	w.status = code
}

// Write implements the http.ResponseWriter interface.
func (w *responseWriter) Write(data []byte) (i int, err error) {
	i, err = w.ResponseWriter.Write(data)
	w.responseSize += i

	return
}

// Unwrap returns the wrapped writer, so http.ResponseController can reach it.
func (w *responseWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

var _ http.Flusher = (*responseWriter)(nil)

// Flush implements the http.Flush interface.
func (w *responseWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

var _ http.Hijacker = (*responseWriter)(nil)

// Hijack implements the http.Hijacker interface.
//
//nolint:revive // the signature is fixed by the interface or the caller
func (w *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}

	return nil, nil, errors.New("not supported")
}

func (w *responseWriter) end() {
	duration := time.Since(w.begin)
	w.rm.Counter("response", w.name, "status", strconv.Itoa(w.status), "count").Inc(1)
	w.rm.Histogram("response", w.name, "size").Update(int64(w.responseSize))
	w.rm.Histogram("response", w.name, "time").Update(int64(duration))
}
