package router

import (
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/9edang/inkgate/engine/config"
)

type recordedStream struct {
	opened, refused, closed int
	code                    int
	toClient, fromClient    int // messages
	bytesTo, bytesFrom      int
}

type recorder struct {
	mu sync.Mutex
	by map[string]*recordedStream
}

func (r *recorder) get(endpoint string) *recordedStream {
	if r.by[endpoint] == nil {
		r.by[endpoint] = &recordedStream{}
	}

	return r.by[endpoint]
}

func (r *recorder) StreamOpened(e, _ string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.get(e).opened++
}

func (r *recorder) StreamRefused(e, _ string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.get(e).refused++
}

func (r *recorder) StreamClosed(e, _ string, _ time.Duration, code int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.get(e).closed++
	r.get(e).code = code
}

func (r *recorder) StreamMessage(e string, toClient bool, n int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s := r.get(e)
	if toClient {
		s.toClient++
		s.bytesTo += n
	} else {
		s.fromClient++
		s.bytesFrom += n
	}
}

func (r *recorder) snapshot(e string) recordedStream {
	r.mu.Lock()
	defer r.mu.Unlock()

	return *r.get(e)
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if cond() {
			return
		}
	}
}

// M3 3.3: the observer sees streams open and close, their chunks and bytes, refusals and socket close codes.
func TestStreamObserver(t *testing.T) {
	rec := &recorder{by: map[string]*recordedStream{}}
	RegisterStreamObserver(rec)
	defer RegisterStreamObserver(nil)

	backend := sseBackend(5, 5*time.Millisecond)
	defer backend.Close()
	gw := streamGateway(backend.URL, config.ExtraConfig{StreamNamespace: map[string]any{}})
	defer gw.Close()
	events, _, _ := readEvents(t, gw.URL)
	waitFor(t, func() bool { return rec.snapshot("/events").closed == 1 })
	sse := rec.snapshot("/events")
	if len(events) != 5 || sse.opened != 1 || sse.closed != 1 || sse.code != 0 || sse.toClient == 0 || sse.bytesTo == 0 {
		t.Errorf("event stream: %+v (events %d)", sse, len(events))
	}

	echo := echoBackend()
	defer echo.Close()
	sgw := socketGateway(echo.URL, map[string]any{"mode": "socket", "max_connections": 1}, 0)
	defer sgw.Close()
	c, _, err := websocket.DefaultDialer.Dial(wsURLOf(sgw), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, resp, err := websocket.DefaultDialer.Dial(wsURLOf(sgw), nil); err == nil || resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("the second socket must be refused: %v", err)
	}
	for _, msg := range []string{"one", "three"} {
		_ = c.WriteMessage(websocket.TextMessage, []byte(msg))
		_, _, _ = c.ReadMessage()
	}
	_ = c.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"), time.Now().Add(time.Second))
	_, _, _ = c.ReadMessage()
	c.Close()
	waitFor(t, func() bool { return rec.snapshot("/ws").closed == 1 })

	ws := rec.snapshot("/ws")
	want := recordedStream{opened: 1, refused: 1, closed: 1, code: websocket.CloseNormalClosure,
		toClient: 2, fromClient: 2, bytesTo: 8, bytesFrom: 8}
	if ws != want {
		t.Errorf("socket: %+v, want %+v", ws, want)
	}
}
