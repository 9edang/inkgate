// SPDX-License-Identifier: Apache-2.0

package router

import (
	"github.com/9edang/inkgate/context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/transport/http/socket"
)

// echoBackend is a WebSocket service: it echoes messages back, picks the
// "chat" subprotocol when offered and closes with the code the client sends
// as a "close:<code>" text message.
func echoBackend() *httptest.Server {
	up := websocket.Upgrader{Subprotocols: []string{"chat"}}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			typ, msg, err := c.ReadMessage()
			if err != nil {
				return
			}
			if typ == websocket.TextMessage && strings.HasPrefix(string(msg), "close:") {
				_ = c.WriteControl(websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "bye"), time.Now().Add(time.Second))
				// wait for the close echo
				c.ReadMessage()
				return
			}
			if err := c.WriteMessage(typ, msg); err != nil {
				return
			}
		}
	}))
}

func socketGateway(backendURL string, extra map[string]any, writeTimeout time.Duration) *httptest.Server {
	u, _ := url.Parse(backendURL)
	prxy := func(ctx context.Context, r *proxy.Request) (*proxy.Response, error) {
		r.URL = u
		if up := socket.From(ctx); up != nil {
			return socket.Proxy(ctx, up, r)
		}
		return nil, nil
	}
	h := EndpointHandler(&config.EndpointConfig{
		Endpoint:    "/ws",
		Method:      "GET",
		Timeout:     150 * time.Millisecond,
		ExtraConfig: config.ExtraConfig{StreamNamespace: extra},
	}, prxy)
	srv := httptest.NewUnstartedServer(h)
	if writeTimeout > 0 {
		srv.Config.WriteTimeout = writeTimeout
	}
	srv.Start()
	return srv
}

func wsURLOf(srv *httptest.Server) string { return "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws" }

func TestSocketEndpoint_relaysMessages(t *testing.T) {
	backend := echoBackend()
	defer backend.Close()
	gw := socketGateway(backend.URL, map[string]any{"mode": "socket"}, 300*time.Millisecond)
	defer gw.Close()

	d := websocket.Dialer{Subprotocols: []string{"other", "chat"}}
	c, resp, err := d.Dial(wsURLOf(gw), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer c.Close()
	if resp.StatusCode != http.StatusSwitchingProtocols {
		t.Errorf("unexpected status %d", resp.StatusCode)
	}
	if resp.Header.Get("X-Inkgate") == "" {
		t.Errorf("the gateway headers must be on the 101 answer: %v", resp.Header)
	}
	if c.Subprotocol() != "chat" {
		t.Errorf("the subprotocol picked by the backend must reach the client, got %q", c.Subprotocol())
	}

	// text and binary keep their type
	for _, tc := range []struct {
		typ int
		msg string
	}{{websocket.TextMessage, "hello"}, {websocket.BinaryMessage, "\x00\x01\x02"}, {websocket.TextMessage, strings.Repeat("x", 100000)}} {
		if err := c.WriteMessage(tc.typ, []byte(tc.msg)); err != nil {
			t.Fatal(err)
		}
		typ, got, err := c.ReadMessage()
		if err != nil || typ != tc.typ || string(got) != tc.msg {
			t.Fatalf("echo mismatch: type %d err %v len %d", typ, err, len(got))
		}
	}

	// the socket outlives the endpoint timeout (150ms) and the server write timeout (300ms)
	time.Sleep(450 * time.Millisecond)
	if err := c.WriteMessage(websocket.TextMessage, []byte("still here")); err != nil {
		t.Fatal(err)
	}
	if _, got, err := c.ReadMessage(); err != nil || string(got) != "still here" {
		t.Fatalf("the socket must outlive the timeouts: %q %v", got, err)
	}
}

func TestSocketEndpoint_pingPongAndCloseAreForwarded(t *testing.T) {
	backend := echoBackend()
	defer backend.Close()
	gw := socketGateway(backend.URL, map[string]any{"mode": "socket"}, 0)
	defer gw.Close()

	c, _, err := websocket.DefaultDialer.Dial(wsURLOf(gw), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	// a ping from the client is answered by the backend (through the gateway)
	pong := make(chan string, 1)
	c.SetPongHandler(func(d string) error { pong <- d; return nil })
	// the echo backend uses gorilla's default ping handler, which answers with a pong
	if err := c.WriteControl(websocket.PingMessage, []byte("beat"), time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	go func() { c.ReadMessage() }() // pong frames are handled while reading
	select {
	case d := <-pong:
		if d != "beat" {
			t.Errorf("unexpected pong payload %q", d)
		}
	case <-time.After(time.Second):
		t.Fatal("the pong of the backend did not reach the client")
	}
}

func TestSocketEndpoint_closeCodeReachesClient(t *testing.T) {
	backend := echoBackend()
	defer backend.Close()
	gw := socketGateway(backend.URL, map[string]any{"mode": "socket"}, 0)
	defer gw.Close()

	c, _, err := websocket.DefaultDialer.Dial(wsURLOf(gw), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.WriteMessage(websocket.TextMessage, []byte("close:now"))
	_, _, err = c.ReadMessage()
	if !websocket.IsCloseError(err, websocket.ClosePolicyViolation) {
		t.Fatalf("expected the close code 1008 of the backend, got %v", err)
	}
	if ce := err.(*websocket.CloseError); ce.Text != "bye" {
		t.Errorf("unexpected close reason %q", ce.Text)
	}
}

func TestSocketEndpoint_plainRequestGets426(t *testing.T) {
	backend := echoBackend()
	defer backend.Close()
	gw := socketGateway(backend.URL, map[string]any{"mode": "socket"}, 0)
	defer gw.Close()

	resp, err := http.Get(gw.URL + "/ws")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUpgradeRequired || resp.Header.Get("Upgrade") != "websocket" {
		t.Errorf("expected 426 with Upgrade: websocket, got %d %v", resp.StatusCode, resp.Header)
	}
}

func TestSocketEndpoint_backendRefusalReachesClient(t *testing.T) {
	refusing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "no way", http.StatusForbidden)
	}))
	defer refusing.Close()
	gw := socketGateway(refusing.URL, map[string]any{"mode": "socket"}, 0)
	defer gw.Close()

	_, resp, err := websocket.DefaultDialer.Dial(wsURLOf(gw), nil)
	if err == nil || resp == nil {
		t.Fatalf("expected the upgrade to be refused, got %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("the backend status must reach the client, got %d", resp.StatusCode)
	}
}

func TestSocketEndpoint_handshakeTimeout(t *testing.T) {
	silent := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
		}
	}))
	defer silent.Close()
	gw := socketGateway(silent.URL, map[string]any{"mode": "socket"}, 0)
	defer gw.Close()

	start := time.Now()
	_, resp, err := websocket.DefaultDialer.Dial(wsURLOf(gw), nil)
	if err == nil {
		t.Fatal("expected the handshake to fail")
	}
	if resp != nil && resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected a 500, got %d", resp.StatusCode)
	}
	if time.Since(start) > time.Second {
		t.Errorf("the endpoint timeout was not applied to the handshake: %s", time.Since(start))
	}
}

func TestSocketEndpoint_idleTimeoutClosesTheSocket(t *testing.T) {
	backend := echoBackend()
	defer backend.Close()
	gw := socketGateway(backend.URL, map[string]any{"mode": "socket", "idle_timeout": "200ms"}, 0)
	defer gw.Close()

	c, _, err := websocket.DefaultDialer.Dial(wsURLOf(gw), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
	start := time.Now()
	if _, _, err := c.ReadMessage(); err == nil {
		t.Fatal("expected the idle socket to be closed")
	}
	if time.Since(start) > time.Second {
		t.Errorf("the idle timeout did not close the socket in time: %s", time.Since(start))
	}
}

func TestParseStream(t *testing.T) {
	for name, tc := range map[string]struct {
		extra config.ExtraConfig
		mode  string
		idle  time.Duration
		max   int64
		fail  bool
	}{
		"absent":       {extra: nil},
		"empty":        {extra: config.ExtraConfig{StreamNamespace: map[string]any{}}, mode: StreamModeEvent},
		"event":        {extra: config.ExtraConfig{StreamNamespace: map[string]any{"mode": "event"}}, mode: StreamModeEvent},
		"socket":       {extra: config.ExtraConfig{StreamNamespace: map[string]any{"mode": "socket"}}, mode: StreamModeSocket},
		"disabled":     {extra: config.ExtraConfig{StreamNamespace: map[string]any{"mode": "socket", "enabled": false}}},
		"options":      {extra: config.ExtraConfig{StreamNamespace: map[string]any{"mode": "socket", "idle_timeout": "30s", "max_message_size": float64(1024)}}, mode: StreamModeSocket, idle: 30 * time.Second, max: 1024},
		"unknown mode": {extra: config.ExtraConfig{StreamNamespace: map[string]any{"mode": "websocket"}}, fail: true},
		"non string":   {extra: config.ExtraConfig{StreamNamespace: map[string]any{"mode": 3}}, fail: true},
		"bad idle":     {extra: config.ExtraConfig{StreamNamespace: map[string]any{"mode": "socket", "idle_timeout": "soon"}}, fail: true},
		"bad max":      {extra: config.ExtraConfig{StreamNamespace: map[string]any{"mode": "socket", "max_message_size": "big"}}, fail: true},
	} {
		got, err := parseStream(&config.EndpointConfig{Endpoint: "/x", ExtraConfig: tc.extra})
		if (err != nil) != tc.fail {
			t.Errorf("%s: unexpected error state: %v", name, err)
			continue
		}
		if err == nil && (got.mode != tc.mode || got.idleTimeout != tc.idle || got.maxMessageSize != tc.max) {
			t.Errorf("%s: got %+v", name, got)
		}
	}
}

func TestEndpointHandler_invalidStreamModePanicsAtStartup(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("an invalid router/stream mode must fail when the handler is built")
		}
	}()
	EndpointHandler(&config.EndpointConfig{
		Endpoint: "/x", Method: "GET",
		ExtraConfig: config.ExtraConfig{StreamNamespace: map[string]any{"mode": "wat"}},
	}, proxy.NoopProxy)
}
