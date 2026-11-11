// SPDX-License-Identifier: Apache-2.0

package router

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"

	"github.com/9edang/inkgate/engine/config"
)

func TestOriginPolicy(t *testing.T) {
	p, err := parseOriginPolicy([]string{"https://app.example.com", "https://*.example.org", "http://localhost:3000"})
	if err != nil {
		t.Fatal(err)
	}
	for origin, want := range map[string]bool{
		"":                                 true, // not a browser
		"https://app.example.com":          true,
		"HTTPS://App.Example.com":          true, // scheme and host are case-insensitive
		"https://app.example.com:443":      true, // the default port
		"https://app.example.com/":         true,
		"http://app.example.com":           false, // another scheme
		"https://app.example.com:8443":     false, // another port
		"https://evil.com":                 false,
		"https://app.example.com.evil.com": false,
		"https://a.example.org":            true,
		"https://a.b.example.org":          true,
		"https://example.org":              false, // the wildcard is for subdomains only
		"https://badexample.org":           false,
		"https://a.example.org:8443":       false,
		"http://localhost:3000":            true,
		"http://localhost":                 false,
		"null":                             false, // sandboxed iframe, file://
		"https://app.example.com/path":     false, // not an origin
		"https://user@app.example.com":     false,
	} {
		if got := p.allows(origin); got != want {
			t.Errorf("allows(%q) = %v, want %v", origin, got, want)
		}
	}

	if !(originPolicy{}).allows("https://evil.com") {
		t.Error("no list must allow every origin")
	}
	none, _ := parseOriginPolicy([]string{})
	if none.allows("https://app.example.com") || !none.allows("") {
		t.Error("an empty list must refuse every browser origin and let clients without Origin through")
	}
	star, _ := parseOriginPolicy([]string{"*"})
	if !star.allows("https://evil.com") {
		t.Error(`"*" must allow every origin`)
	}

	for _, bad := range []string{"app.example.com", "https://app.example.com/x", "ftp://example.com", "https://*", "https://a.*.example.com", "*.example.com"} {
		if _, err := parseOriginPolicy([]string{bad}); err == nil {
			t.Errorf("%q must be refused", bad)
		}
	}
}

func TestSubprotocolPolicy(t *testing.T) {
	p := subprotocolPolicy{"graphql-transport-ws", "chat"}
	for _, tc := range []struct {
		offered, want []string
		ok            bool
	}{
		{nil, nil, true},
		{[]string{"chat", "other", "graphql-transport-ws"}, []string{"chat", "graphql-transport-ws"}, true}, // client order kept
		{[]string{"other"}, nil, false},
		{[]string{"Chat"}, nil, false}, // case-sensitive
	} {
		got, ok := p.filter(tc.offered)
		if ok != tc.ok || !slices.Equal(got, tc.want) {
			t.Errorf("filter(%v) = %v, %v; want %v, %v", tc.offered, got, ok, tc.want, tc.ok)
		}
	}
	if got, ok := subprotocolPolicy(nil).filter([]string{"x"}); !ok || !slices.Equal(got, []string{"x"}) {
		t.Errorf("no list must pass everything, got %v %v", got, ok)
	}
}

func TestParseStream_socketPolicy(t *testing.T) {
	parse := func(m map[string]any) (streamConfig, error) {
		return parseStream(&config.EndpointConfig{Endpoint: "/x", ExtraConfig: config.ExtraConfig{StreamNamespace: m}})
	}
	sc, err := parse(map[string]any{"mode": "socket", "allowed_origins": []any{"https://a.com"}, "subprotocols": []any{"chat"}})
	if err != nil {
		t.Fatal(err)
	}
	if !sc.origins.enabled() || !slices.Equal(sc.subprotocols, subprotocolPolicy{"chat"}) {
		t.Errorf("unexpected config %+v", sc)
	}
	for name, m := range map[string]map[string]any{
		"origin not a list":     {"mode": "socket", "allowed_origins": "https://a.com"},
		"origin not a string":   {"mode": "socket", "allowed_origins": []any{1}},
		"origin with a path":    {"mode": "socket", "allowed_origins": []any{"https://a.com/x"}},
		"empty subprotocol":     {"mode": "socket", "subprotocols": []any{""}},
		"origins on event mode": {"mode": "event", "allowed_origins": []any{"https://a.com"}},
		"subprotocols on event": {"subprotocols": []any{"chat"}},
	} {
		if _, err := parse(m); err == nil {
			t.Errorf("%s: must fail", name)
		}
	}
}

// countingEcho is echoBackend that counts the upgrade requests it receives and records the subprotocols offered.
func countingEcho(hits *atomic.Int32, offered *atomic.Value) *httptest.Server {
	inner := echoBackend()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		offered.Store(websocket.Subprotocols(r))
		// the echo backend checks Origin against its own host, like gorilla's default; here only the gateway decides
		r.Header.Del("Origin")
		inner.Config.Handler.ServeHTTP(w, r)
	}))
}

func TestSocketEndpoint_allowedOrigins(t *testing.T) {
	var hits atomic.Int32
	var offered atomic.Value
	backend := countingEcho(&hits, &offered)
	defer backend.Close()
	gw := socketGateway(backend.URL, map[string]any{"mode": "socket", "allowed_origins": []any{"https://app.example.com"}}, 0)
	defer gw.Close()

	dial := func(origin string) (int, error) {
		h := http.Header{}
		if origin != "" {
			h.Set("Origin", origin)
		}
		c, resp, err := websocket.DefaultDialer.Dial(wsURLOf(gw), h)
		if c != nil {
			defer c.Close()
			if err := c.WriteMessage(websocket.TextMessage, []byte("hi")); err != nil {
				return 0, err
			}
			if _, msg, err := c.ReadMessage(); err != nil || string(msg) != "hi" {
				t.Errorf("origin %q: echo %q %v", origin, msg, err)
			}
		}
		if resp == nil {
			return 0, err
		}
		return resp.StatusCode, nil
	}

	if code, err := dial("https://app.example.com"); err != nil || code != http.StatusSwitchingProtocols {
		t.Fatalf("allowed origin: %d %v", code, err)
	}
	if code, err := dial(""); err != nil || code != http.StatusSwitchingProtocols {
		t.Fatalf("no origin: %d %v", code, err)
	}
	before := hits.Load()
	if code, _ := dial("https://evil.com"); code != http.StatusForbidden {
		t.Fatalf("other origin: got %d, want 403", code)
	}
	if hits.Load() != before {
		t.Error("the backend was dialed for a refused origin")
	}
}

func TestSocketEndpoint_subprotocols(t *testing.T) {
	var hits atomic.Int32
	var offered atomic.Value
	backend := countingEcho(&hits, &offered)
	defer backend.Close()
	gw := socketGateway(backend.URL, map[string]any{"mode": "socket", "subprotocols": []any{"chat"}}, 0)
	defer gw.Close()

	d := websocket.Dialer{Subprotocols: []string{"admin", "chat"}}
	c, _, err := d.Dial(wsURLOf(gw), nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	c.Close()
	if c.Subprotocol() != "chat" {
		t.Errorf("negotiated %q, want chat", c.Subprotocol())
	}
	if got := offered.Load().([]string); !slices.Equal(got, []string{"chat"}) {
		t.Errorf("the backend was offered %v, want only [chat]", got)
	}

	before := hits.Load()
	d = websocket.Dialer{Subprotocols: []string{"admin"}}
	_, resp, err := d.Dial(wsURLOf(gw), nil)
	if err == nil || resp == nil || resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("no allowed subprotocol: want 400, got %v %v", resp, err)
	}
	if hits.Load() != before {
		t.Error("the backend was dialed without an allowed subprotocol")
	}

	// a client that offers none still connects: the subprotocol is optional
	c, _, err = websocket.DefaultDialer.Dial(wsURLOf(gw), nil)
	if err != nil {
		t.Fatalf("no subprotocol: %v", err)
	}
	c.Close()
}

// A backend that answers with a subprotocol nobody offered is refused, also without a subprotocols list.
func TestSocketEndpoint_backendSubprotocolNotOffered(t *testing.T) {
	up := websocket.Upgrader{} // picks no subprotocol by itself: the response header decides
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, http.Header{"Sec-Websocket-Protocol": {"rogue"}})
		if err == nil {
			c.Close()
		}
	}))
	defer backend.Close()
	gw := socketGateway(backend.URL, map[string]any{"mode": "socket"}, 0)
	defer gw.Close()

	for _, offer := range [][]string{nil, {"chat"}} {
		d := websocket.Dialer{Subprotocols: offer}
		c, resp, err := d.Dial(wsURLOf(gw), nil)
		if err == nil {
			c.Close()
			t.Fatalf("offer %v: the client was upgraded with %q", offer, c.Subprotocol())
		}
		if resp == nil || resp.StatusCode == http.StatusSwitchingProtocols {
			t.Errorf("offer %v: want an error status, got %v", offer, resp)
		}
	}
}
