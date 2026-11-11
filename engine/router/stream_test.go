// SPDX-License-Identifier: Apache-2.0

package router

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/9edang/inkgate/context"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/transport/http/client"
)

// sseBackend sends n events, one every gap, the way an SSE service does.
func sseBackend(n int, gap time.Duration) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		f := w.(http.Flusher)
		f.Flush()
		for i := range n {
			select {
			case <-r.Context().Done():
				return
			case <-time.After(gap):
			}
			fmt.Fprintf(w, "id: %d\ndata: event-%d\n\n", i, i)
			f.Flush()
		}
	}))
}

// streamGateway serves one endpoint in front of the backend, with a short
// endpoint timeout and server write timeout: both are shorter than the stream.
func streamGateway(backendURL string, extra config.ExtraConfig) *httptest.Server {
	u, _ := url.Parse(backendURL)
	httpProxy := proxy.NewHTTPProxyDetailed(&config.Backend{},
		client.DefaultHTTPRequestExecutor(client.NewHTTPClient),
		client.DefaultHTTPStatusHandler, proxy.NoOpHTTPResponseParser)
	prxy := func(ctx context.Context, r *proxy.Request) (*proxy.Response, error) {
		r.URL = u
		return httpProxy(ctx, r)
	}
	h := EndpointHandler(&config.EndpointConfig{
		Endpoint:       "/events",
		Method:         "GET",
		Timeout:        150 * time.Millisecond,
		OutputEncoding: "no-op",
		Backend:        []*config.Backend{{Encoding: "no-op"}},
		ExtraConfig:    extra,
	}, prxy)

	srv := httptest.NewUnstartedServer(h)
	srv.Config.WriteTimeout = 300 * time.Millisecond
	srv.Start()
	return srv
}

func readEvents(t *testing.T, url string) (events []string, arrivals []time.Time, resp *http.Response) {
	t.Helper()
	req, _ := http.NewRequest("GET", url, http.NoBody)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		if after, ok := strings.CutPrefix(sc.Text(), "data: "); ok {
			events = append(events, after)
			arrivals = append(arrivals, time.Now())
		}
	}
	if err := sc.Err(); err != nil {
		t.Errorf("reading the stream: %v", err)
	}
	return events, arrivals, resp
}

func TestStreamEndpoint_SSE(t *testing.T) {
	backend := sseBackend(5, 120*time.Millisecond) // 600ms of stream, well over both timeouts
	defer backend.Close()
	gw := streamGateway(backend.URL, config.ExtraConfig{StreamNamespace: map[string]any{}})
	defer gw.Close()

	start := time.Now()
	events, arrivals, resp := readEvents(t, gw.URL+"/events")

	if len(events) != 5 {
		t.Fatalf("expected the 5 events to reach the client, got %v", events)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("unexpected content type %q", ct)
	}
	if resp.Header.Get("Cache-Control") != "no-cache" || resp.Header.Get("X-Accel-Buffering") != "no" {
		t.Errorf("missing streaming headers: %v", resp.Header)
	}
	// events are flushed as they come, not delivered together at the end
	if got := arrivals[0].Sub(start); got > 400*time.Millisecond {
		t.Errorf("first event arrived after %s: the response is buffered", got)
	}
	if gap := arrivals[len(arrivals)-1].Sub(arrivals[0]); gap < 300*time.Millisecond {
		t.Errorf("events arrived together (%s apart): the response is buffered", gap)
	}
}

func TestStreamEndpoint_headerTimeoutStillApplies(t *testing.T) {
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer slow.Close()
	gw := streamGateway(slow.URL, config.ExtraConfig{StreamNamespace: map[string]any{}})
	defer gw.Close()

	start := time.Now()
	resp, err := http.Get(gw.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected a 500 when the backend does not answer in time, got %d", resp.StatusCode)
	}
	if time.Since(start) > time.Second {
		t.Errorf("the endpoint timeout was not applied before the headers: %s", time.Since(start))
	}
}

func TestStreamEndpoint_notConfiguredIsCutByTimeout(t *testing.T) {
	backend := sseBackend(5, 120*time.Millisecond)
	defer backend.Close()
	gw := streamGateway(backend.URL, nil)
	defer gw.Close()

	events, _, _ := readEvents(t, gw.URL+"/events")
	if len(events) >= 5 {
		t.Errorf("without router/stream the endpoint timeout must cut the stream, got %d events", len(events))
	}
}

func TestStreamEndpoint_clientLeavingStopsBackend(t *testing.T) {
	stopped := make(chan struct{})
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		f := w.(http.Flusher)
		for {
			select {
			case <-r.Context().Done():
				close(stopped)
				return
			case <-time.After(20 * time.Millisecond):
				fmt.Fprint(w, "data: tick\n\n")
				f.Flush()
			}
		}
	}))
	defer backend.Close()
	gw := streamGateway(backend.URL, config.ExtraConfig{StreamNamespace: map[string]any{}})
	defer gw.Close()

	resp, err := http.Get(gw.URL + "/events")
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 16)
	if _, err := resp.Body.Read(buf); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close() // the client leaves

	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Error("the backend stream was not closed after the client left")
	}
}

// max_message_size is read whatever JSON decoder built the config: koanf gives float64, internal/json gives int64.
func TestParseStreamMaxMessageSizeAnyNumberType(t *testing.T) {
	for _, size := range []any{float64(1024), int64(1024), 1024} {
		cfg := &config.EndpointConfig{Endpoint: "/s", ExtraConfig: config.ExtraConfig{
			StreamNamespace: map[string]any{"max_message_size": size},
		}}
		sc, err := parseStream(cfg)
		if err != nil {
			t.Fatalf("%T: %v", size, err)
		}
		if sc.maxMessageSize != 1024 {
			t.Errorf("%T: maxMessageSize %d, want 1024", size, sc.maxMessageSize)
		}
	}
}
