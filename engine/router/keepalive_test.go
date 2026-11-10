package router

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/9edang/inkgate/engine/config"
)

// scriptedBackend writes the parts in order, flushing each, with the pauses between them.
func scriptedBackend(contentType string, parts ...any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(http.StatusOK)
		f := w.(http.Flusher)
		f.Flush()
		for _, p := range parts {
			switch v := p.(type) {
			case string:
				_, _ = io.WriteString(w, v)
				f.Flush()
			case time.Duration:
				time.Sleep(v)
			}
		}
	}))
}

func streamBody(t *testing.T, backend *httptest.Server, extra map[string]any) string {
	t.Helper()
	gw := streamGateway(backend.URL, config.ExtraConfig{StreamNamespace: extra})
	defer gw.Close()
	resp, err := http.Get(gw.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)

	return string(b)
}

// M3 3.4: a silent SSE stream gets keep-alive comments between events; the events are intact.
func TestKeepAlive_SSE(t *testing.T) {
	backend := scriptedBackend("text/event-stream", "data: one\n\n", 350*time.Millisecond, "data: two\n\n")
	defer backend.Close()
	body := streamBody(t, backend, map[string]any{"keep_alive": "100ms"})

	if n := strings.Count(body, ": keep-alive\n\n"); n < 2 {
		t.Errorf("%d keep-alive comments in a 350 ms silence at 100 ms, want 2 or more:\n%q", n, body)
	}
	if strings.ReplaceAll(body, ": keep-alive\n\n", "") != "data: one\n\ndata: two\n\n" {
		t.Errorf("the events changed: %q", body)
	}
}

// Never inside an event: a pause in the middle of one gets no comment, which would break it.
func TestKeepAlive_notInsideAnEvent(t *testing.T) {
	backend := scriptedBackend("text/event-stream", "data: hel", 350*time.Millisecond, "lo\n\n")
	defer backend.Close()
	if body := streamBody(t, backend, map[string]any{"keep_alive": "100ms"}); body != "data: hello\n\n" {
		t.Errorf("body %q, want the event untouched", body)
	}
}

// Only for SSE: other chunked streams (NDJSON...) would be corrupted by a comment line.
func TestKeepAlive_onlyForEventStreams(t *testing.T) {
	backend := scriptedBackend("application/x-ndjson", "{\"a\":1}\n", 350*time.Millisecond, "{\"a\":2}\n")
	defer backend.Close()
	if body := streamBody(t, backend, map[string]any{"keep_alive": "100ms"}); body != "{\"a\":1}\n{\"a\":2}\n" {
		t.Errorf("body %q, want the stream untouched", body)
	}
}

// Without keep_alive nothing is added, as before.
func TestKeepAlive_offByDefault(t *testing.T) {
	backend := scriptedBackend("text/event-stream", "data: one\n\n", 250*time.Millisecond, "data: two\n\n")
	defer backend.Close()
	if body := streamBody(t, backend, map[string]any{}); body != "data: one\n\ndata: two\n\n" {
		t.Errorf("body %q", body)
	}
}

// With keep-alive the backend is read on another goroutine: a client that leaves must still end the backend stream.
func TestKeepAlive_clientLeavingStopsBackend(t *testing.T) {
	stopped := make(chan struct{})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		w.(http.Flusher).Flush()
		_, _ = io.WriteString(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		<-r.Context().Done() // then silent until the gateway closes it
		close(stopped)
	}))
	defer backend.Close()
	gw := streamGateway(backend.URL, config.ExtraConfig{StreamNamespace: map[string]any{"keep_alive": "50ms"}})
	defer gw.Close()

	resp, err := http.Get(gw.URL)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 64)
	_, _ = resp.Body.Read(buf)
	time.Sleep(120 * time.Millisecond) // a couple of keep-alives
	resp.Body.Close()                  // the client leaves

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Error("the backend stream was not closed after the client left")
	}
}

func TestParseStreamKeepAlive(t *testing.T) {
	sc, err := parseStream(&config.EndpointConfig{Endpoint: "/x", ExtraConfig: config.ExtraConfig{
		StreamNamespace: map[string]any{"keep_alive": "15s"},
	}})
	if err != nil || sc.keepAlive != 15*time.Second {
		t.Errorf("got %v, %v", sc.keepAlive, err)
	}
	for _, raw := range []any{"soon", "0s", "-1s", 15} {
		if _, err := parseStream(&config.EndpointConfig{Endpoint: "/x", ExtraConfig: config.ExtraConfig{
			StreamNamespace: map[string]any{"keep_alive": raw},
		}}); err == nil {
			t.Errorf("keep_alive %v must be refused at startup", raw)
		}
	}
}

// The end of an event is found also when its blank line is split across chunks.
func TestEventBoundary(t *testing.T) {
	for name, tc := range map[string]struct {
		chunks  []string
		between bool
	}{
		"nothing yet":           {nil, true},
		"whole event":           {[]string{"data: a\n\n"}, true},
		"mid event":             {[]string{"data: a"}, false},
		"one newline":           {[]string{"data: a\n"}, false},
		"split blank line":      {[]string{"data: a\n", "\n"}, true},
		"crlf split":            {[]string{"data: a\r\n\r", "\n"}, true},
		"tiny chunks":           {[]string{"d", "a", "\n", "\n"}, true},
		"next event started":    {[]string{"data: a\n\n", "data: b"}, false},
		"cr only":               {[]string{"data: a\r\r"}, true},
		"long chunk ending mid": {[]string{strings.Repeat("x", 100) + "\n\ndata: b"}, false},
	} {
		var e eventBoundary
		for _, c := range tc.chunks {
			e.wrote([]byte(c))
		}
		if e.between() != tc.between {
			t.Errorf("%s: between = %v, want %v", name, e.between(), tc.between)
		}
	}
}
