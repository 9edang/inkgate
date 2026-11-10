package metrics

import (
	"strconv"
	"strings"
	"time"

	"github.com/9edang/inkgate/context"

	"github.com/rcrowley/go-metrics"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/transport/http/client"
)

// NewProxyMiddleware creates a proxy middleware ready to be injected in the pipe as instrumentation point
func (m *Metrics) NewProxyMiddleware(layer, name string) proxy.Middleware {
	return NewProxyMiddleware(layer, name, m.Proxy)
}

// ProxyFactory creates an instrumented proxy factory
func (m *Metrics) ProxyFactory(segmentName string, next proxy.Factory) proxy.FactoryFunc {
	if m.Config == nil || m.Config.ProxyDisabled {
		return next.New
	}
	return proxy.FactoryFunc(func(cfg *config.EndpointConfig) (proxy.Proxy, error) {
		next, err := next.New(cfg)
		if err != nil {
			return proxy.NoopProxy, err
		}
		return m.NewProxyMiddleware(segmentName, cfg.Endpoint)(next), nil
	})
}

// BackendFactory creates an instrumented backend factory
func (m *Metrics) BackendFactory(segmentName string, next proxy.BackendFactory) proxy.BackendFactory {
	if m.Config == nil || m.Config.BackendDisabled {
		return next
	}
	return func(cfg *config.Backend) proxy.Proxy {
		return m.NewProxyMiddleware(segmentName, cfg.URLPattern)(next(cfg))
	}
}

// DefaultBackendFactory creates an instrumented default HTTP backend factory
func (m *Metrics) DefaultBackendFactory() proxy.BackendFactory {
	return m.BackendFactory("backend", proxy.CustomHTTPProxyFactory(client.NewHTTPClient))
}

// NewProxyMetrics creates a ProxyMetrics using the injected registry
func NewProxyMetrics(parent *metrics.Registry) *ProxyMetrics {
	m := metrics.NewPrefixedChildRegistry(*parent, "proxy.")
	return &ProxyMetrics{m}
}

// NewProxyMiddleware creates a proxy middleware ready to be injected in the pipe as instrumentation point
func NewProxyMiddleware(layer, name string, pm *ProxyMetrics) proxy.Middleware {
	handles := registerProxyMiddlewareMetrics(layer, name, pm)
	return func(next ...proxy.Proxy) proxy.Proxy {
		if len(next) > 1 {
			panic(proxy.ErrTooManyProxies)
		}
		return func(ctx context.Context, request *proxy.Request) (*proxy.Response, error) {
			begin := time.Now()
			resp, err := next[0](ctx, request)

			// the metrics are resolved once per middleware; updating them is cheap and safe for concurrent use,
			// so it happens inline instead of in a goroutine per call
			h := handles[outcome(resp != nil && resp.IsComplete, err != nil)]
			h.requests.Inc(1)
			h.latency.Update(time.Since(begin).Nanoseconds())

			return resp, err
		}
	}
}

// proxyHandles are the request counter and the latency histogram of one outcome of a proxy.
type proxyHandles struct {
	requests metrics.Counter
	latency  metrics.Histogram
}

// outcome indexes the four (complete, errored) combinations.
func outcome(complete, errored bool) int {
	i := 0
	if complete {
		i |= 1
	}
	if errored {
		i |= 2
	}

	return i
}

func registerProxyMiddlewareMetrics(layer, name string, pm *ProxyMetrics) [4]proxyHandles {
	var handles [4]proxyHandles
	labels := "layer." + layer + ".name." + name
	for _, complete := range []bool{true, false} {
		for _, errored := range []bool{true, false} {
			suffix := labels + ".complete." + strconv.FormatBool(complete) + ".error." + strconv.FormatBool(errored)
			handles[outcome(complete, errored)] = proxyHandles{
				requests: pm.Counter("requests." + suffix),
				latency:  pm.Histogram("latency." + suffix),
			}
		}
	}

	return handles
}

// ProxyMetrics is the metrics collector for the proxy package
type ProxyMetrics struct {
	register metrics.Registry
}

// Histogram gets or register a histogram. The sample (about 8 KB) is built only when the histogram does not exist
// yet: go-metrics calls a function argument lazily.
func (rm *ProxyMetrics) Histogram(labels ...string) metrics.Histogram {
	r := rm.register
	if r == nil {
		r = metrics.DefaultRegistry // as metrics.GetOrRegisterHistogram does: disabled metrics have no registry
	}
	if h, ok := r.GetOrRegister(strings.Join(labels, "."), newHistogram).(metrics.Histogram); ok {
		return h
	}

	return metrics.NilHistogram{} // the name is taken by another metric type
}

func newHistogram() metrics.Histogram {
	return metrics.NewHistogram(defaultSample())
}

// Counter gets or register a counter
func (rm *ProxyMetrics) Counter(labels ...string) metrics.Counter {
	return metrics.GetOrRegisterCounter(strings.Join(labels, "."), rm.register)
}
