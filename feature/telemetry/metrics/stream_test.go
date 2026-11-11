package metrics

import (
	"testing"
	"time"

	gometrics "github.com/rcrowley/go-metrics"

	"github.com/9edang/inkgate/engine/router"
)

var _ router.StreamObserver = (*StreamMetrics)(nil)

func TestStreamMetrics(t *testing.T) {
	r := gometrics.NewRegistry()
	s := NewStreamMetrics(NewRouterMetrics(&r))

	s.StreamOpened("/ws", "socket")
	s.StreamOpened("/ws", "socket")
	s.StreamRefused("/ws", "socket")
	s.StreamMessage("/ws", true, 10)
	s.StreamMessage("/ws", false, 4)
	s.StreamMessage("/ws", false, 6)
	s.StreamClosed("/ws", "socket", time.Second, 1001)

	counter := func(name string) int64 {
		c, ok := r.Get("router.stream./ws." + name).(gometrics.Counter)
		if !ok {
			t.Fatalf("missing metric router.stream./ws.%s", name)
		}

		return c.Count()
	}
	for name, want := range map[string]int64{
		"open": 1, "opened": 2, "refused": 1, "close.1001": 1,
		"messages.to_client": 1, "messages.from_client": 2, "bytes.to_client": 10, "bytes.from_client": 10,
	} {
		if got := counter(name); got != want {
			t.Errorf("%s = %d, want %d", name, got, want)
		}
	}
	if h, ok := r.Get("router.stream./ws.duration").(gometrics.Histogram); !ok || h.Count() != 1 {
		t.Error("duration not recorded")
	}
}

// Called once per message on every open socket: recording one must not allocate.
func TestStreamMessageDoesNotAllocate(t *testing.T) {
	r := gometrics.NewRegistry()
	s := NewStreamMetrics(NewRouterMetrics(&r))
	s.StreamOpened("/ws", "socket") // resolves the endpoint's metrics
	if n := testing.AllocsPerRun(1000, func() { s.StreamMessage("/ws", true, 64) }); n != 0 {
		t.Errorf("%v allocations per message, want 0", n)
	}
}
