package backend

import (
	"time"

	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/proxy"
)

// recordFlow is step 0 of the backend stage: it records every call to a backend
// in the flow context and, when propagate_request_id is on, adds the request ID
// header to the backend request. Being the outermost step, the recorded duration
// covers the whole backend stage (circuit breaker, rate limit, Lua, connectors, HTTP).
func recordFlow(next proxy.BackendFactory) proxy.BackendFactory {
	return func(remote *config.Backend) proxy.Proxy {
		call := next(remote)
		return func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
			flow := context.From(ctx)
			if flow == nil {
				return call(ctx, req)
			}
			if flow.PropagatesRequestID() && req != nil {
				headers := make(map[string][]string, len(req.Headers)+1)
				for k, v := range req.Headers {
					headers[k] = v
				}
				headers[flow.RequestIDHeader()] = []string{flow.RequestID()}
				clone := *req
				clone.Headers = headers
				req = &clone
			}

			start := time.Now()
			resp, err := call(ctx, req)

			rec := context.BackendCall{
				Backend:  remote.URLPattern,
				Method:   remote.Method,
				Hosts:    remote.Host,
				Duration: time.Since(start),
			}
			if resp != nil {
				rec.Status = resp.Metadata.StatusCode
				rec.Complete = resp.IsComplete
			}
			if err != nil {
				rec.Err = err.Error()
			}
			flow.AddBackendCall(rec)
			return resp, err
		}
	}
}
