// SPDX-License-Identifier: Apache-2.0

package router

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/9edang/inkgate/engine/config"
)

func TestParseStream_messageRate(t *testing.T) {
	parse := func(m map[string]any) (streamConfig, error) {
		return parseStream(&config.EndpointConfig{Endpoint: "/x", ExtraConfig: config.ExtraConfig{StreamNamespace: m}})
	}
	for name, tc := range map[string]struct {
		m     map[string]any
		rate  float64
		burst int
	}{
		"none":            {m: map[string]any{"mode": "socket"}},
		"default burst":   {m: map[string]any{"mode": "socket", "max_messages_per_second": 10.0}, rate: 10, burst: 10},
		"slow, burst 1":   {m: map[string]any{"mode": "socket", "max_messages_per_second": 0.5}, rate: 0.5, burst: 1},
		"fractional rate": {m: map[string]any{"mode": "socket", "max_messages_per_second": 2.5}, rate: 2.5, burst: 3},
		"explicit burst":  {m: map[string]any{"mode": "socket", "max_messages_per_second": 10.0, "message_burst": 50.0}, rate: 10, burst: 50},
	} {
		sc, err := parse(tc.m)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if sc.messageRate != tc.rate || sc.messageBurst != tc.burst {
			t.Errorf("%s: rate %v burst %d, want %v %d", name, sc.messageRate, sc.messageBurst, tc.rate, tc.burst)
		}
	}
	for name, m := range map[string]map[string]any{
		"zero rate":        {"mode": "socket", "max_messages_per_second": 0.0},
		"negative rate":    {"mode": "socket", "max_messages_per_second": -1.0},
		"quoted rate":      {"mode": "socket", "max_messages_per_second": "10"},
		"burst without":    {"mode": "socket", "message_burst": 5.0},
		"zero burst":       {"mode": "socket", "max_messages_per_second": 1.0, "message_burst": 0.0},
		"fractional burst": {"mode": "socket", "max_messages_per_second": 1.0, "message_burst": 1.5},
		"rate on event":    {"mode": "event", "max_messages_per_second": 1.0},
	} {
		if _, err := parse(m); err == nil {
			t.Errorf("%s: must fail", name)
		}
	}
}

// expectPolicyClose reads until the connection ends and checks it ended with 1008. Messages read before are returned.
func expectPolicyClose(t *testing.T, c *websocket.Conn) []string {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	var got []string
	for {
		_, msg, err := c.ReadMessage()
		if err == nil {
			got = append(got, string(msg))
			continue
		}
		var ce *websocket.CloseError
		if !errors.As(err, &ce) || ce.Code != websocket.ClosePolicyViolation {
			t.Fatalf("want close 1008, got %v (after %v)", err, got)
		}

		return got
	}
}

// recordingBackend is a WebSocket service that sends every message it receives, and then how its connection ended,
// on the returned channel.
func recordingBackend() (*httptest.Server, <-chan string) {
	got := make(chan string, 16)
	up := websocket.Upgrader{}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, err := up.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer c.Close()
		for {
			_, msg, err := c.ReadMessage()
			if err != nil {
				got <- "end: " + err.Error()
				close(got)
				return
			}
			got <- string(msg)
		}
	})), got
}

// Over the rate the socket is closed with 1008 on both sides, and the message over it never reaches the backend.
func TestSocketEndpoint_messageRateClosesWith1008(t *testing.T) {
	backend, received := recordingBackend()
	defer backend.Close()
	gw := socketGateway(backend.URL, map[string]any{"mode": "socket", "max_messages_per_second": 0.1, "message_burst": 3.0}, 0)
	defer gw.Close()

	c, _, err := websocket.DefaultDialer.Dial(wsURLOf(gw), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for i := range 4 {
		if err := c.WriteMessage(websocket.TextMessage, []byte(strconv.Itoa(i))); err != nil {
			t.Fatal(err)
		}
	}
	expectPolicyClose(t, c)

	var msgs []string
	for m := range received {
		msgs = append(msgs, m)
	}
	want := []string{"0", "1", "2", "end: websocket: close 1008 (policy violation): message rate exceeded"}
	if !slices.Equal(msgs, want) {
		t.Errorf("the backend received %q, want %q", msgs, want)
	}
}

// Pings count: a ping flood is closed like a message flood.
func TestSocketEndpoint_messageRateCountsPings(t *testing.T) {
	backend := echoBackend()
	defer backend.Close()
	gw := socketGateway(backend.URL, map[string]any{"mode": "socket", "max_messages_per_second": 0.1, "message_burst": 2.0}, 0)
	defer gw.Close()

	c, _, err := websocket.DefaultDialer.Dial(wsURLOf(gw), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for range 3 {
		if err := c.WriteControl(websocket.PingMessage, nil, time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
	}
	expectPolicyClose(t, c)
}

// A client within the rate is never cut, also past its burst.
func TestSocketEndpoint_messageRateWithinLimit(t *testing.T) {
	backend := echoBackend()
	defer backend.Close()
	gw := socketGateway(backend.URL, map[string]any{"mode": "socket", "max_messages_per_second": 50.0, "message_burst": 1.0}, 0)
	defer gw.Close()

	c, _, err := websocket.DefaultDialer.Dial(wsURLOf(gw), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	for i := range 5 {
		if i > 0 {
			time.Sleep(60 * time.Millisecond) // three times the 20 ms a token takes
		}
		want := strconv.Itoa(i)
		if err := c.WriteMessage(websocket.TextMessage, []byte(want)); err != nil {
			t.Fatal(err)
		}
		if _, msg, err := c.ReadMessage(); err != nil || string(msg) != want {
			t.Fatalf("message %d: %q %v", i, msg, err)
		}
	}
}
