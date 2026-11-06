package opencensus

import (
	"net/http"

	"github.com/9edang/inkgate/context"

	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/tag"
	"go.opencensus.io/trace"

	"github.com/9edang/inkgate/engine/config"
	transport "github.com/9edang/inkgate/engine/transport/http/client"
)

var defaultClient = &http.Client{Transport: &ochttp.Transport{}}

func NewHTTPClient(ctx context.Context) *http.Client {
	if !IsBackendEnabled() {
		return transport.NewHTTPClient(ctx)
	}

	return defaultClient
}

func HTTPRequestExecutor(clientFactory transport.HTTPClientFactory) transport.HTTPRequestExecutor {
	return HTTPRequestExecutorFromConfig(clientFactory, nil)
}

func HTTPRequestExecutorFromConfig(clientFactory transport.HTTPClientFactory, cfg *config.Backend) transport.HTTPRequestExecutor {
	if !IsBackendEnabled() {
		return transport.DefaultHTTPRequestExecutor(clientFactory)
	}

	pathExtractor := GetAggregatedPathForBackendMetrics(cfg)

	return func(ctx context.Context, req *http.Request) (*http.Response, error) {
		httpClient := clientFactory(ctx)

		if _, ok := httpClient.Transport.(*Transport); ok {
			// #nosec G704 -- the target URL comes from the operator config
			return httpClient.Do(req.WithContext(trace.NewContext(ctx, fromContext(ctx))))
		}

		c := &http.Client{
			Transport: &Transport{
				Base: httpClient.Transport,
				tags: []tagGenerator{
					func(r *http.Request) tag.Mutator { return tag.Upsert(ochttp.KeyClientHost, req.Host) },
					func(r *http.Request) tag.Mutator { return tag.Upsert(ochttp.KeyClientPath, pathExtractor(r)) },
					func(r *http.Request) tag.Mutator { return tag.Upsert(ochttp.KeyClientMethod, req.Method) },
				},
			},
			CheckRedirect: httpClient.CheckRedirect,
			Jar:           httpClient.Jar,
			Timeout:       httpClient.Timeout,
		}
		// #nosec G704 -- the target URL comes from the operator config
		return c.Do(req.WithContext(trace.NewContext(ctx, fromContext(ctx))))
	}
}
