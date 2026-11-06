// SPDX-License-Identifier: Apache-2.0

/*
Package client provides some http helpers to create http clients and executors
*/
package client

import (
	"net/http"

	"github.com/9edang/inkgate/context"
)

// HTTPRequestExecutor defines the interface of the request executor for the HTTP transport protocol.
type HTTPRequestExecutor func(ctx context.Context, req *http.Request) (*http.Response, error)

// DefaultHTTPRequestExecutor creates a HTTPRequestExecutor with the received HTTPClientFactory.
func DefaultHTTPRequestExecutor(clientFactory HTTPClientFactory) HTTPRequestExecutor {
	return func(ctx context.Context, req *http.Request) (*http.Response, error) {
		// #nosec G704 -- the target URL comes from the operator config
		return clientFactory(ctx).Do(req.WithContext(ctx))
	}
}

// HTTPClientFactory creates http clients based with the received context.
type HTTPClientFactory func(ctx context.Context) *http.Client

// NewHTTPClient just returns the http default client.
func NewHTTPClient(_ context.Context) *http.Client { return defaultHTTPClient }

var defaultHTTPClient = &http.Client{}
