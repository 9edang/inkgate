package proxy

import (
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/encoding"
	"github.com/9edang/inkgate/internal/json"
)

func backendResponse(status int, body []byte, gzipped bool) *http.Response {
	w := httptest.NewRecorder()
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Set-Cookie", "backend=secret")
	if gzipped {
		w.Header().Set("Content-Encoding", "gzip")
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write(body)
		_ = zw.Close()
		body = buf.Bytes()
	}
	w.WriteHeader(status)
	_, _ = w.Write(body)

	return w.Result()
}

func parsers() (decoded, raw HTTPResponseParser) {
	cfg := HTTPResponseParserConfig{Decoder: encoding.JSONDecoder, EntityFormatter: NewEntityFormatter(&config.Backend{})}

	return DefaultHTTPResponseParserFactory(cfg), RawJSONHTTPResponseParserFactory(cfg)
}

// The fast path gives what today's path gives: same status, completeness and headers, and a body with the same
// JSON value; for anything that is not a plain non-empty object, the very same result or error.
func TestRawJSONParserMatchesDecoded(t *testing.T) {
	decoded, raw := parsers()
	for name, body := range map[string]string{
		"object":       `{"id":42,"name":"Ada","tags":["a","b"],"nested":{"x":1.5,"y":null}}`,
		"unicode":      `{"text":"héllo é 😀","<html>":"&"}`,
		"numbers":      `{"big":9007199254740993,"neg":-1,"exp":1e2,"float":0.1}`,
		"spaces":       " \n\t{ \"a\" : 1 } \n",
		"empty object": `{}`,
		"null":         `null`,
		"array":        `[1,2,3]`,
		"invalid":      `{"a":`,
		"empty body":   ``,
	} {
		for _, gzipped := range []bool{false, true} {
			want, wantErr := decoded(context.Background(), backendResponse(201, []byte(body), gzipped))
			got, gotErr := raw(context.Background(), backendResponse(201, []byte(body), gzipped))

			if (wantErr == nil) != (gotErr == nil) {
				t.Errorf("%s (gzip %v): error %v, want %v", name, gzipped, gotErr, wantErr)
				continue
			}
			if wantErr != nil {
				if gotErr.Error() != wantErr.Error() {
					t.Errorf("%s (gzip %v): error %q, want %q", name, gzipped, gotErr, wantErr)
				}
				continue
			}
			if got.IsComplete != want.IsComplete || !reflect.DeepEqual(got.Metadata, want.Metadata) {
				t.Errorf("%s (gzip %v): got %+v, want %+v", name, gzipped, got, want)
			}
			if got.Raw != nil && len(got.Data) != 0 {
				t.Errorf("%s: raw bytes must come with empty data", name)
			}

			wBody, wErr := want.JSONBody()
			gBody, gErr := got.JSONBody()
			if wErr != nil || gErr != nil {
				t.Fatalf("%s: encoding: %v / %v", name, gErr, wErr)
			}
			var wv, gv any
			_ = json.Unmarshal(wBody, &wv)
			_ = json.Unmarshal(gBody, &gv)
			if !reflect.DeepEqual(wv, gv) {
				t.Errorf("%s (gzip %v): body %s, want %s", name, gzipped, gBody, wBody)
			}
		}
	}
}

// The raw bytes are used only while nothing changed the data: a change made by any feature wins.
func TestJSONBodyPrefersChangedData(t *testing.T) {
	body, err := (&Response{Data: map[string]any{"changed": true}, Raw: []byte(`{"original":true}`)}).JSONBody()
	if err != nil || string(body) != `{"changed":true}` {
		t.Errorf("body %s, err %v", body, err)
	}
	body, _ = (&Response{Data: map[string]any{}, Raw: []byte(`{"original":true}`)}).JSONBody()
	if string(body) != `{"original":true}` {
		t.Errorf("unchanged data must give the raw bytes, got %s", body)
	}
}

func passthroughEndpoint(change func(e *config.EndpointConfig, b *config.Backend)) *config.EndpointConfig {
	b := &config.Backend{Encoding: encoding.JSON, ExtraConfig: config.ExtraConfig{}}
	e := &config.EndpointConfig{OutputEncoding: encoding.JSON, Backend: []*config.Backend{b}, ExtraConfig: config.ExtraConfig{}}
	if change != nil {
		change(e, b)
	}

	return e
}

func TestJSONPassthroughDecision(t *testing.T) {
	off := map[string]func(e *config.EndpointConfig, b *config.Backend){
		"two backends":     func(e *config.EndpointConfig, b *config.Backend) { e.Backend = append(e.Backend, b) },
		"output xml":       func(e *config.EndpointConfig, _ *config.Backend) { e.OutputEncoding = "xml" },
		"output negotiate": func(e *config.EndpointConfig, _ *config.Backend) { e.OutputEncoding = "negotiate" },
		"backend no-op":    func(_ *config.EndpointConfig, b *config.Backend) { b.Encoding = encoding.NOOP },
		"backend safejson": func(_ *config.EndpointConfig, b *config.Backend) { b.Encoding = encoding.SAFE_JSON },
		"allow":            func(_ *config.EndpointConfig, b *config.Backend) { b.AllowList = []string{"a"} },
		"deny":             func(_ *config.EndpointConfig, b *config.Backend) { b.DenyList = []string{"a"} },
		"mapping":          func(_ *config.EndpointConfig, b *config.Backend) { b.Mapping = map[string]string{"a": "b"} },
		"group":            func(_ *config.EndpointConfig, b *config.Backend) { b.Group = "g" },
		"target":           func(_ *config.EndpointConfig, b *config.Backend) { b.Target = "t" },
		"is_collection":    func(_ *config.EndpointConfig, b *config.Backend) { b.IsCollection = true },
	}
	// every namespace that reads or changes the body, and one nobody knows (a plugin)
	for _, ns := range []string{
		"inkgate/engine/proxy", "inkgate/engine/proxy/plugin", "inkgate/engine/router/stream",
		"inkgate/feature/validation/cel", "inkgate/feature/auth/jose/signer", "inkgate/feature/transform/martian",
		"inkgate/feature/transform/lua/router", "inkgate/feature/transform/lua/proxy",
		"inkgate/engine/transport/http/client/plugin", "inkgate/engine/transport/http/client/graphql",
		"some/unknown-plugin",
	} {
		off["endpoint "+ns] = func(e *config.EndpointConfig, _ *config.Backend) { e.ExtraConfig[ns] = map[string]any{} }
	}
	off["backend inkgate/feature/transform/lua/proxy/backend"] = func(_ *config.EndpointConfig, b *config.Backend) {
		b.ExtraConfig["inkgate/feature/transform/lua/proxy/backend"] = map[string]any{}
	}
	for name, change := range off {
		if ok, reason := JSONPassthrough(passthroughEndpoint(change)); ok || reason == "" {
			t.Errorf("%s: pass-through on, want off with a reason", name)
		}
	}

	if ok, reason := JSONPassthrough(passthroughEndpoint(nil)); !ok {
		t.Errorf("a plain JSON endpoint: off (%s), want on", reason)
	}
	if ok, _ := JSONPassthrough(passthroughEndpoint(func(_ *config.EndpointConfig, b *config.Backend) { b.Encoding = "" })); !ok {
		t.Error("no backend encoding means JSON: want on")
	}
	for ns := range passthroughNamespaces {
		e := passthroughEndpoint(func(e *config.EndpointConfig, b *config.Backend) {
			e.ExtraConfig[ns] = map[string]any{}
			b.ExtraConfig[ns] = map[string]any{}
		})
		if ok, reason := JSONPassthrough(e); !ok {
			t.Errorf("safe namespace %s turned it off (%s)", ns, reason)
		}
	}
}
