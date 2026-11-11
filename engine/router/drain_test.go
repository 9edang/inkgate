package router

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/transport/http/client"
	"github.com/9edang/inkgate/engine/transport/http/server"
	"github.com/9edang/inkgate/engine/transport/http/socket"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	return l.Addr().(*net.TCPAddr).Port
}

// runGateway serves handler through server.RunServer, the way the binary does, and returns its address and a
// function that shuts it down like SIGTERM and reports how long the shutdown took.
func runGateway(t *testing.T, handler http.Handler) (addr string, shutdown func() (time.Duration, error)) {
	t.Helper()
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- server.RunServer(ctx, config.ServiceConfig{Port: port, MaxShutdownDuration: 5 * time.Second}, handler)
	}()
	addr = fmt.Sprintf("localhost:%d", port)
	for range 200 {
		if c, err := net.Dial("tcp", addr); err == nil {
			c.Close()
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	return addr, func() (time.Duration, error) {
		start := time.Now()
		cancel()
		select {
		case err := <-done:
			return time.Since(start), err
		case <-time.After(10 * time.Second):
			return time.Since(start), errors.New("the server did not stop")
		}
	}
}

// M3 3.2: on shutdown every WebSocket client gets a close frame 1001 (going away), and the shutdown waits for them.
func TestDrain_SocketsGetGoingAway(t *testing.T) {
	rec := &recorder{by: map[string]*recordedStream{}}
	RegisterStreamObserver(rec)
	defer RegisterStreamObserver(nil)
	backend := echoBackend()
	defer backend.Close()
	u, _ := url.Parse(backend.URL)
	h := EndpointHandler(&config.EndpointConfig{
		Endpoint: "/ws", Method: "GET", Timeout: time.Second,
		ExtraConfig: config.ExtraConfig{StreamNamespace: map[string]any{"mode": "socket"}},
	}, func(ctx context.Context, r *proxy.Request) (*proxy.Response, error) {
		r.URL = u
		if up := socket.From(ctx); up != nil {
			return socket.Proxy(ctx, up, r)
		}
		return nil, nil
	})
	addr, shutdown := runGateway(t, h)

	var clients []*websocket.Conn
	for range 3 {
		c, _, err := websocket.DefaultDialer.Dial("ws://"+addr+"/ws", nil)
		if err != nil {
			t.Fatal(err)
		}
		clients = append(clients, c)
	}

	codes := make(chan int, len(clients))
	for _, c := range clients {
		_ = c.SetReadDeadline(time.Now().Add(5 * time.Second)) // a socket left open fails the test instead of hanging
		go func() {
			for {
				if _, _, err := c.ReadMessage(); err != nil {
					if ce, ok := errors.AsType[*websocket.CloseError](err); ok {
						codes <- ce.Code
					} else {
						codes <- -1 // dropped without a close frame
					}
					return
				}
			}
		}()
	}

	took, err := shutdown()
	if err != nil {
		t.Fatal(err)
	}
	for range clients {
		if code := <-codes; code != websocket.CloseGoingAway {
			t.Errorf("client got close code %d, want %d (going away)", code, websocket.CloseGoingAway)
		}
	}
	if took > 3*time.Second {
		t.Errorf("shutdown took %v", took)
	}
	if got := rec.snapshot("/ws"); got.closed != 3 || got.code != websocket.CloseGoingAway {
		t.Errorf("observed %+v, want 3 sockets closed with 1001", got)
	}
}

// On shutdown an open SSE stream ends cleanly (the client reads to EOF, no error) and does not hold the shutdown.
func TestDrain_SSEEndsCleanly(t *testing.T) {
	backend := sseBackend(1000, 20*time.Millisecond) // 20 s of events: longer than the test
	defer backend.Close()
	u, _ := url.Parse(backend.URL)
	httpProxy := proxy.NewHTTPProxyDetailed(&config.Backend{},
		client.DefaultHTTPRequestExecutor(client.NewHTTPClient),
		client.DefaultHTTPStatusHandler, proxy.NoOpHTTPResponseParser)
	h := EndpointHandler(&config.EndpointConfig{
		Endpoint: "/events", Method: "GET", Timeout: time.Second, OutputEncoding: "no-op",
		Backend:     []*config.Backend{{Encoding: "no-op"}},
		ExtraConfig: config.ExtraConfig{StreamNamespace: map[string]any{}},
	}, func(ctx context.Context, r *proxy.Request) (*proxy.Response, error) {
		r.URL = u
		return httpProxy(ctx, r)
	})
	addr, shutdown := runGateway(t, h)

	resp, err := http.Get("http://" + addr + "/events")
	if err != nil {
		t.Fatal(err)
	}
	ended := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(resp.Body)
		events := 0
		for sc.Scan() {
			if strings.HasPrefix(sc.Text(), "data: ") {
				events++
			}
		}
		resp.Body.Close()
		if err := sc.Err(); err != nil {
			ended <- err
			return
		}
		if events == 0 {
			ended <- io.ErrUnexpectedEOF
			return
		}
		ended <- nil
	}()
	time.Sleep(100 * time.Millisecond) // a few events

	took, err := shutdown()
	if err != nil {
		t.Fatal(err)
	}
	if err := <-ended; err != nil {
		t.Errorf("the stream did not end cleanly: %v", err)
	}
	if took > 3*time.Second {
		t.Errorf("the open stream held the shutdown for %v", took)
	}
}
