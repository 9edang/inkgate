package amqp_test

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/encoding"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	connector "github.com/9edang/inkgate/feature/connector/amqp"
)

const (
	consumerNamespace = "inkgate/feature/connector/amqp/consume"
	producerNamespace = "inkgate/feature/connector/amqp/produce"
)

var totalIterations = flag.Int("iterations", 200, "The number of produce and consume iterations")

// TestProduceConsume publishes messages through the producer backend and reads them back with the consumer backend.
func TestProduceConsume(t *testing.T) {
	host := rabbitmqURL(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	buf := new(bytes.Buffer)
	l, _ := logging.NewLogger("DEBUG", buf, "")
	t.Cleanup(func() {
		if t.Failed() {
			t.Log(buf.String())
		}
	})

	bf := connector.NewBackendFactory(ctx, l, func(_ *config.Backend) proxy.Proxy {
		t.Error("this backend factory shouldn't be called")
		return proxy.NoopProxy
	})

	consumerProxy := bf(&config.Backend{
		Host: []string{host},
		ExtraConfig: config.ExtraConfig{
			consumerNamespace: map[string]any{
				"name":           "queue-1",
				"exchange":       "some-exchange",
				"durable":        true,
				"delete":         false,
				"exclusive":      false,
				"no_wait":        true,
				"auto_ack":       false,
				"no_local":       false,
				"routing_key":    []string{"#"},
				"prefetch_count": 10,
			},
		},
		Decoder: encoding.JSONDecoder,
	})

	producerProxy := bf(&config.Backend{
		Host: []string{host},
		ExtraConfig: config.ExtraConfig{
			producerNamespace: map[string]any{
				"name":      "queue-1",
				"exchange":  "some-exchange",
				"durable":   true,
				"delete":    false,
				"exclusive": false,
				"no_wait":   true,
				"mandatory": true,
				"immediate": false,
			},
		},
	})

	for i := 0; i < *totalIterations; i++ {
		resp, err := producerProxy(ctx, &proxy.Request{
			Headers: map[string][]string{"Content-Type": {"application/json"}},
			Params:  map[string]string{"routing_key": "some_value"},
			Body:    io.NopCloser(bytes.NewBufferString(fmt.Sprintf(`{"foo":"bar","some":%d}`, i))),
		})
		if err != nil {
			t.Fatalf("#%d: cannot produce: %s", i, err)
		}
		if resp == nil || !resp.IsComplete {
			t.Fatalf("#%d: unexpected producer response %v", i, resp)
		}
	}

	for i := 0; i < *totalIterations; i++ {
		localCtx, cancelRead := context.WithTimeout(ctx, 5*time.Second)
		resp, err := consumerProxy(localCtx, nil)
		cancelRead()
		if err != nil {
			t.Fatalf("#%d: unexpected consumer error: %s", i, err)
		}
		if resp == nil || !resp.IsComplete {
			t.Fatalf("#%d: unexpected consumer response %v", i, resp)
		}
		if v, ok := resp.Data["foo"].(string); !ok || v != "bar" {
			t.Fatalf("#%d: unexpected consumer data %v", i, resp.Data)
		}
	}
}
