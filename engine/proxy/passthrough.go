package proxy

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"

	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/encoding"
	"github.com/9edang/inkgate/internal/json"
)

// JSONBody is the JSON the client gets: the backend's bytes when the response came through the pass-through and
// nothing changed its data since, otherwise the data encoded.
func (r *Response) JSONBody() ([]byte, error) {
	if r.Raw != nil && len(r.Data) == 0 {
		return r.Raw, nil
	}

	return json.Marshal(r.Data)
}

// passthroughNamespaces are the extra_config namespaces known to leave a response body alone: they act on the
// request, on the raw HTTP exchange or only observe. Anything else, a plugin included, keeps the decoded path. A
// namespace is added here deliberately, with a test.
var passthroughNamespaces = map[string]bool{
	// endpoint
	"inkgate/feature/telemetry/metrics":     true,
	"inkgate/feature/telemetry/opencensus":  true,
	"telemetry/opentelemetry":               true,
	"qos/ratelimit/router":                  true,
	"inkgate/feature/auth/jose/validator":   true,
	"inkgate/feature/security/botdetector":  true,
	"inkgate/feature/validation/jsonschema": true,
	// backend
	"qos/ratelimit/proxy":                              true,
	"inkgate/feature/traffic/circuitbreaker/gobreaker": true,
	"inkgate/feature/traffic/httpcache":                true,
	"inkgate/engine/transport/http/client":             true,
	"inkgate/feature/auth/oauth2client":                true,
}

// JSONPassthrough decides whether the JSON answer of the endpoint's backend can go to the client as the backend sent
// it, without being decoded and encoded again. When it cannot, the reason says what needs the decoded data.
func JSONPassthrough(e *config.EndpointConfig) (ok bool, reason string) {
	if len(e.Backend) != 1 {
		return false, "more than one backend"
	}
	if e.OutputEncoding != encoding.JSON {
		return false, "output_encoding " + e.OutputEncoding
	}
	b := e.Backend[0]
	switch {
	case b.Encoding != encoding.JSON && b.Encoding != "": // no encoding means JSON
		return false, "backend encoding " + b.Encoding
	case len(b.AllowList) > 0:
		return false, "allow"
	case len(b.DenyList) > 0:
		return false, "deny"
	case len(b.Mapping) > 0:
		return false, "mapping"
	case b.Group != "":
		return false, "group"
	case b.Target != "":
		return false, "target"
	case b.IsCollection:
		return false, "is_collection"
	}
	for _, extra := range []config.ExtraConfig{e.ExtraConfig, b.ExtraConfig} {
		for namespace := range extra {
			if !passthroughNamespaces[namespace] {
				return false, namespace
			}
		}
	}

	return true, ""
}

// RawJSONHTTPResponseParserFactory parses like DefaultHTTPResponseParserFactory, without decoding a valid, non-empty
// JSON object: its bytes go into Response.Raw. Anything else (an empty object, null, an array, invalid JSON) is
// decoded with cfg from the same bytes, so its result or error is exactly the decoded path's.
func RawJSONHTTPResponseParserFactory(cfg HTTPResponseParserConfig) HTTPResponseParser {
	return func(_ context.Context, resp *http.Response) (*Response, error) {
		defer resp.Body.Close()

		var reader io.Reader = resp.Body
		if resp.Header.Get("Content-Encoding") == "gzip" {
			gz, err := gzip.NewReader(resp.Body)
			if err != nil {
				return nil, err
			}
			defer gz.Close()
			reader = gz
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			return nil, err
		}

		if nonEmptyObject(body) && json.Valid(body) {
			return &Response{
				Data:       map[string]any{},
				Raw:        body,
				IsComplete: true,
				Metadata:   Metadata{StatusCode: resp.StatusCode},
			}, nil
		}

		var data map[string]any
		if err := cfg.Decoder(bytes.NewReader(body), &data); err != nil {
			return nil, err
		}
		r := cfg.EntityFormatter.Format(Response{Data: data, IsComplete: true, Metadata: Metadata{StatusCode: resp.StatusCode}})

		return &r, nil
	}
}

// nonEmptyObject reports whether b starts like a JSON object with at least one member.
func nonEmptyObject(b []byte) bool {
	b = bytes.TrimLeft(b, " \t\r\n")
	if len(b) == 0 || b[0] != '{' {
		return false
	}
	b = bytes.TrimLeft(b[1:], " \t\r\n")

	return len(b) > 0 && b[0] != '}'
}
