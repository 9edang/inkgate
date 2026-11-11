// SPDX-License-Identifier: Apache-2.0

package proxy

import (
	"compress/gzip"
	"io"
	"net/http"

	"github.com/9edang/inkgate/context"

	"github.com/9edang/inkgate/engine/encoding"
)

// HTTPResponseParser defines how the response is parsed from http.Response to Response object
type HTTPResponseParser func(context.Context, *http.Response) (*Response, error)

// DefaultHTTPResponseParserConfig defines a default HTTPResponseParserConfig
var DefaultHTTPResponseParserConfig = HTTPResponseParserConfig{
	func(_ io.Reader, _ *map[string]any) error { return nil },
	EntityFormatterFunc(func(r Response) Response { return r }),
}

// HTTPResponseParserConfig contains the config for a given HttpResponseParser
type HTTPResponseParserConfig struct {
	Decoder         encoding.Decoder
	EntityFormatter EntityFormatter
}

// HTTPResponseParserFactory creates HTTPResponseParser from a given HTTPResponseParserConfig
type HTTPResponseParserFactory func(HTTPResponseParserConfig) HTTPResponseParser

// DefaultHTTPResponseParserFactory is the default implementation of HTTPResponseParserFactory
func DefaultHTTPResponseParserFactory(cfg HTTPResponseParserConfig) HTTPResponseParser {
	return func(_ context.Context, resp *http.Response) (*Response, error) {
		defer resp.Body.Close()

		var reader io.ReadCloser
		switch resp.Header.Get("Content-Encoding") {
		case "gzip":
			gzipReader, err := gzip.NewReader(resp.Body)
			if err != nil {
				return nil, err
			}
			reader = gzipReader
			defer reader.Close()
		default:
			reader = resp.Body
		}

		var data map[string]any
		if err := cfg.Decoder(reader, &data); err != nil {
			return nil, err
		}

		// the status is reported, the backend headers are not: the body is encoded again, so headers such as
		// Content-Length would no longer be true
		newResponse := Response{Data: data, IsComplete: true, Metadata: Metadata{StatusCode: resp.StatusCode}}
		newResponse = cfg.EntityFormatter.Format(newResponse)
		return &newResponse, nil
	}
}

// NoOpHTTPResponseParser is a HTTPResponseParser implementation that just copies the
// http response body into the proxy response IO
func NoOpHTTPResponseParser(ctx context.Context, resp *http.Response) (*Response, error) {
	return &Response{
		Data:       map[string]any{},
		IsComplete: true,
		Io:         NewReadCloserWrapper(ctx, resp.Body),
		Metadata: Metadata{
			StatusCode: resp.StatusCode,
			Headers:    resp.Header,
		},
	}, nil
}
