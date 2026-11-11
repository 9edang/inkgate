package router

import (
	"bufio"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/9edang/inkgate/engine/config"
)

// openSSE starts a stream and returns once its first event arrived; the rest of the events go to the channel.
func openSSE(t *testing.T, url string) (events <-chan string, resp *http.Response) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, http.NoBody)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	ch := make(chan string, 100)
	first := make(chan struct{})
	go func() {
		defer close(ch)
		defer resp.Body.Close()
		sc := bufio.NewScanner(resp.Body)
		once := false
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "data: ") {
				ch <- sc.Text()
				if !once {
					once = true
					close(first)
				}
			}
		}
		if !once {
			close(first)
		}
	}()
	<-first

	return ch, resp
}

// M3 3.1: the stream over max_connections is refused with 503 before the backend is called; the open ones go on.
func TestStreamLimit_SSE(t *testing.T) {
	backend := sseBackend(20, 20*time.Millisecond)
	defer backend.Close()
	gw := streamGateway(backend.URL, config.ExtraConfig{StreamNamespace: map[string]any{"max_connections": 2}})
	defer gw.Close()

	a, ra := openSSE(t, gw.URL)
	b, rb := openSSE(t, gw.URL)
	if ra.StatusCode != http.StatusOK || rb.StatusCode != http.StatusOK {
		t.Fatalf("the first two streams: %d, %d", ra.StatusCode, rb.StatusCode)
	}

	extra, err := http.Get(gw.URL)
	if err != nil {
		t.Fatal(err)
	}
	extra.Body.Close()
	if extra.StatusCode != http.StatusServiceUnavailable || extra.Header.Get("Retry-After") == "" {
		t.Errorf("the stream over the limit: status %d, Retry-After %q; want 503 with Retry-After",
			extra.StatusCode, extra.Header.Get("Retry-After"))
	}

	for name, ch := range map[string]<-chan string{"first": a, "second": b} {
		n := 0
		for range ch {
			n++
		}
		if n != 20 {
			t.Errorf("%s stream got %d events, want 20", name, n)
		}
	}

	// both ended: the slots are free again
	_, rc := openSSE(t, gw.URL)
	if rc.StatusCode != http.StatusOK {
		t.Errorf("a stream after the others ended: %d, want 200", rc.StatusCode)
	}
}

func TestStreamLimit_Socket(t *testing.T) {
	backend := echoBackend()
	defer backend.Close()
	gw := socketGateway(backend.URL, map[string]any{"mode": "socket", "max_connections": 1}, 0)
	defer gw.Close()

	first, _, err := websocket.DefaultDialer.Dial(wsURLOf(gw), nil)
	if err != nil {
		t.Fatal(err)
	}

	_, resp, err := websocket.DefaultDialer.Dial(wsURLOf(gw), nil)
	if err == nil || resp == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("the socket over the limit: err %v, resp %v; want a refused upgrade with 503", err, resp)
	}

	if err := first.WriteMessage(websocket.TextMessage, []byte("still here")); err != nil {
		t.Fatal(err)
	}
	if _, msg, err := first.ReadMessage(); err != nil || string(msg) != "still here" {
		t.Fatalf("the open socket stopped working: %q, %v", msg, err)
	}
	first.Close()

	// the slot is freed when the relay ends, shortly after the close
	deadline := time.Now().Add(2 * time.Second)
	for {
		c, _, err := websocket.DefaultDialer.Dial(wsURLOf(gw), nil)
		if err == nil {
			c.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no slot after the first socket closed: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestParseStreamMaxConnections(t *testing.T) {
	for raw, want := range map[any]int{float64(10): 10, int64(3): 3, 7: 7, float64(0): 0} {
		sc, err := parseStream(&config.EndpointConfig{Endpoint: "/x", ExtraConfig: config.ExtraConfig{
			StreamNamespace: map[string]any{"max_connections": raw},
		}})
		if err != nil || sc.maxConnections != want {
			t.Errorf("%T(%v): got %d, %v; want %d", raw, raw, sc.maxConnections, err, want)
		}
	}
	for _, raw := range []any{float64(-1), 2.5, "10", true} {
		if _, err := parseStream(&config.EndpointConfig{Endpoint: "/x", ExtraConfig: config.ExtraConfig{
			StreamNamespace: map[string]any{"max_connections": raw},
		}}); err == nil {
			t.Errorf("%T(%v) must be refused at startup", raw, raw)
		}
	}
}
