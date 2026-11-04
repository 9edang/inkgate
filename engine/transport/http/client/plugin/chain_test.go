// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"errors"
	"github.com/9edang/inkgate/context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/transport/http/client"
)

var done []string

func registerTagger(name string) {
	registerClientNext(name, func(_ context.Context, extra map[string]any, next http.Handler) (http.Handler, error) {
		cfg, _ := extra[name].(map[string]any)
		tag, _ := cfg["tag"].(string)
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			req.Header.Add("X-Chain", tag)
			next.ServeHTTP(w, req)
			done = append(done, tag)
		}), nil
	})
}

func backendEcho(cfg *config.Backend) client.HTTPRequestExecutor {
	return func(_ context.Context, req *http.Request) (*http.Response, error) {
		rec := httptest.NewRecorder()
		rec.WriteString("chain=" + strings.Join(req.Header.Values("X-Chain"), ","))
		return rec.Result(), nil
	}
}

func runChain(t *testing.T, extra map[string]any, next func(*config.Backend) client.HTTPRequestExecutor) (*http.Response, error) {
	t.Helper()
	l, _ := logging.NewLogger("DEBUG", io.Discard, "")
	h := HTTPRequestExecutor(l, next)(&config.Backend{ExtraConfig: map[string]any{Namespace: extra}})
	req, _ := http.NewRequest("GET", "http://some.example.tld/path", http.NoBody)
	return h(context.Background(), req)
}

func TestHTTPRequestExecutor_chain(t *testing.T) {
	registerTagger("chain-one")
	registerTagger("chain-two")
	done = nil

	resp, err := runChain(t, map[string]any{
		"name":      []any{"chain-one", "chain-two"},
		"chain-one": map[string]any{"tag": "one"},
		"chain-two": map[string]any{"tag": "two"},
	}, backendEcho)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "chain=one,two" {
		t.Errorf("unexpected body %q: the first plugin of the list must run first", b)
	}
	// the response goes back through the plugins in reverse order
	if got := strings.Join(done, ","); got != "two,one" {
		t.Errorf("unexpected unwinding order %q", got)
	}
}

func TestHTTPRequestExecutor_singleMiddlewareAsString(t *testing.T) {
	registerTagger("chain-single")
	resp, err := runChain(t, map[string]any{
		"name":         "chain-single",
		"chain-single": map[string]any{"tag": "solo"},
	}, backendEcho)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "chain=solo" {
		t.Errorf("unexpected body %q", b)
	}
}

func TestHTTPRequestExecutor_plainPluginReplacesBackend(t *testing.T) {
	RegisterClient("chain-plain", func(_ context.Context, _ map[string]any) (http.Handler, error) {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Write([]byte("plain"))
		}), nil
	})
	registerTagger("chain-before-plain")

	resp, err := runChain(t, map[string]any{
		"name":               []any{"chain-before-plain", "chain-plain"},
		"chain-before-plain": map[string]any{"tag": "x"},
	}, func(*config.Backend) client.HTTPRequestExecutor {
		t.Error("the backend should not be called")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "plain" {
		t.Errorf("unexpected body %q", b)
	}
}

func TestHTTPRequestExecutor_backendErrorPropagates(t *testing.T) {
	registerTagger("chain-err")
	want := errors.New("backend down")
	resp, err := runChain(t, map[string]any{
		"name":      []any{"chain-err"},
		"chain-err": map[string]any{"tag": "e"},
	}, func(*config.Backend) client.HTTPRequestExecutor {
		return func(context.Context, *http.Request) (*http.Response, error) { return nil, want }
	})
	if err != want || resp != nil {
		t.Errorf("expected the backend error, got resp=%v err=%v", resp, err)
	}
}

func TestHTTPRequestExecutor_unknownNamesFallBack(t *testing.T) {
	registerTagger("chain-known")
	resp, err := runChain(t, map[string]any{
		"name": []any{"chain-nope", "chain-known", 42},
	}, backendEcho)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "chain=" {
		t.Errorf("unexpected body %q", b)
	}

	called := false
	_, _ = runChain(t, map[string]any{"name": []any{"chain-nope"}}, func(cfg *config.Backend) client.HTTPRequestExecutor {
		called = true
		return backendEcho(cfg)
	})
	if !called {
		t.Error("with no usable plugin the regular backend must be used")
	}
}
