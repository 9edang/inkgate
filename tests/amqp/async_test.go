package amqp_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/encoding"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/feature/connector/amqp/async"
	amqp "github.com/rabbitmq/amqp091-go"
)

const agentNamespace = "inkgate/feature/connector/amqp/agent"

func publish(host, exchange string, iterations int, content string) error {
	conn, err := amqp.Dial(host)
	if err != nil {
		return fmt.Errorf("cannot publish to %s: %w", host, err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("cannot get a channel: %w", err)
	}
	defer ch.Close()

	body := fmt.Sprintf(`{"msg":"%s"}`, content)
	for i := 0; i < iterations; i++ {
		err = ch.Publish(exchange, "foo.bar", false, false, amqp.Publishing{
			ContentType: "application/json",
			Body:        []byte(body),
		})
		if err != nil {
			return fmt.Errorf("cannot publish on exchange %s: %w", exchange, err)
		}
	}

	return nil
}

// fakeHandler is the backend the agent calls for every message it consumes.
type fakeHandler struct {
	Received    chan int
	data        [][]byte
	numReceived int
	mu          sync.RWMutex
}

func newFakeHandler() *fakeHandler {
	return &fakeHandler{Received: make(chan int, 100), data: make([][]byte, 0, 16)}
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	_, _ = w.Write(b)

	h.mu.Lock()
	h.numReceived++
	n := h.numReceived
	h.data = append(h.data, b)
	h.mu.Unlock()

	h.Received <- n
}

func (h *fakeHandler) Data(atIdx int) []byte {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.data) > atIdx {
		return h.data[atIdx]
	}

	return nil
}

// newAgent returns the subscriber config and options of an agent that forwards every message to backend.
func newAgent(t *testing.T, ctx context.Context, host, backend string, prefetch, maxRate int) (async.Subscriber, async.Options, func()) {
	t.Helper()

	cfg := async.Subscriber{
		Topic: "#",
		Endpoint: &config.EndpointConfig{
			Backend: []*config.Backend{{
				URLPattern: "/__debug/",
				Host:       []string{backend},
				Decoder:    encoding.JSONDecoder,
			}},
		},
		Timeout: time.Second,
		ExtraConfig: config.ExtraConfig{
			agentNamespace: map[string]any{
				"name":           "inkgate-test",
				"host":           host + "/",
				"exchange":       "inkgate-testing",
				"delete":         true,
				"no_wait":        true,
				"prefetch_count": prefetch,
			},
		},
		Workers: 1,
		MaxRate: float64(maxRate),
	}

	buf := new(bytes.Buffer)
	l, _ := logging.NewLogger("DEBUG", buf, "")
	t.Cleanup(func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	})

	p, err := proxy.DefaultFactory(l).New(cfg.Endpoint)
	if err != nil {
		t.Fatalf("cannot create the proxy: %s", err)
	}

	// keep reading the pings of the worker, or the subscriber blocks on its first one
	pings := make(chan string)
	go func() {
		for {
			select {
			case <-pings:
			case <-ctx.Done():
				return
			}
		}
	}()

	ticker := time.NewTicker(time.Second)

	return cfg, async.Options{Logger: l, Proxy: p, Ping: pings, PingTicker: ticker}, ticker.Stop
}

func TestAgentNotRateLimited(t *testing.T) {
	host := rabbitmqURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	h := newFakeHandler()
	backend := httptest.NewServer(h)
	defer backend.Close()

	cfg, opts, stop := newAgent(t, ctx, host, backend.URL, 0, 0)
	defer stop()

	// New blocks while the agent runs
	agentErr := make(chan error, 1)
	go func() { agentErr <- async.New(ctx, cfg, opts) }()

	time.Sleep(time.Second) // let the agent declare its queue

	if err := publish(host, "inkgate-testing", 1, "hello 0"); err != nil {
		t.Fatalf("cannot publish: %s", err)
	}

	select {
	case <-time.After(6 * time.Second):
		t.Fatal("the backend received nothing")
	case <-h.Received:
	}

	cancel()
	if err := <-agentErr; err != nil && ctx.Err() == nil {
		t.Errorf("the agent failed: %s", err)
	}

	if got, want := string(h.Data(0)), `{"msg":"hello 0"}`; got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestAgentRateLimited(t *testing.T) {
	host := rabbitmqURL(t)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	h := newFakeHandler()
	backend := httptest.NewServer(h)
	defer backend.Close()

	cfg, opts, stop := newAgent(t, ctx, host, backend.URL, 1, 1)
	defer stop()

	go func() {
		if err := async.New(ctx, cfg, opts); err != nil && ctx.Err() == nil {
			t.Errorf("the agent exited with an error: %s", err)
		}
	}()

	time.Sleep(time.Second) // let the agent declare its queue

	go func() {
		for i := 0; i < 10; i++ {
			if err := publish(host, "inkgate-testing", 1, fmt.Sprintf("hello %d", i)); err != nil {
				t.Errorf("cannot publish: %s", err)
			}
		}
	}()

	processed := 0
	deadline := time.After(3 * time.Second)
loop:
	for {
		select {
		case <-deadline:
			break loop
		case <-h.Received:
			processed++
		}
	}

	cancel()
	// max_rate 1: the bucket starts with one token and gains one per second, so at most 4 in 3 seconds
	// (without the limit all 10 would be processed)
	if processed > 4 {
		t.Errorf("processed %d messages in 3 seconds, the rate limit is 1 per second", processed)
	}
}
