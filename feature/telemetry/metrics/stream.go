package metrics

import (
	"strconv"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
)

// StreamMetrics implements router.StreamObserver: the streams and sockets of each streaming endpoint, under
// router.stream.<endpoint>.*: open (now), opened, refused, duration (ns), messages.to_client / messages.from_client,
// bytes.to_client / bytes.from_client and close.<code>. The metrics of an endpoint are resolved once, not looked up by
// name on every message.
type StreamMetrics struct {
	rm *RouterMetrics
	by sync.Map // endpoint -> *streamHandles
}

// NewStreamMetrics returns the stream metrics of the router registry.
func NewStreamMetrics(rm *RouterMetrics) *StreamMetrics {
	return &StreamMetrics{rm: rm}
}

type streamHandles struct {
	rm                                           *RouterMetrics
	endpoint                                     string
	open, opened, refused                        metrics.Counter
	duration                                     metrics.Histogram
	messagesTo, messagesFrom, bytesTo, bytesFrom metrics.Counter
	codes                                        sync.Map // close code -> metrics.Counter
}

func (s *StreamMetrics) handles(endpoint string) *streamHandles {
	if h, ok := s.by.Load(endpoint); ok {
		if handles, ok := h.(*streamHandles); ok {
			return handles
		}
	}
	c := func(name ...string) metrics.Counter {
		return s.rm.Counter(append([]string{"stream", endpoint}, name...)...)
	}
	h, _ := s.by.LoadOrStore(endpoint, &streamHandles{
		rm:           s.rm,
		endpoint:     endpoint,
		open:         c("open"),
		opened:       c("opened"),
		refused:      c("refused"),
		duration:     s.rm.Histogram("stream", endpoint, "duration"),
		messagesTo:   c("messages", "to_client"),
		messagesFrom: c("messages", "from_client"),
		bytesTo:      c("bytes", "to_client"),
		bytesFrom:    c("bytes", "from_client"),
	})
	handles, _ := h.(*streamHandles) // only *streamHandles are stored

	return handles
}

// StreamOpened implements router.StreamObserver.
func (s *StreamMetrics) StreamOpened(endpoint, _ string) {
	h := s.handles(endpoint)
	h.open.Inc(1)
	h.opened.Inc(1)
}

// StreamRefused implements router.StreamObserver.
func (s *StreamMetrics) StreamRefused(endpoint, _ string) {
	s.handles(endpoint).refused.Inc(1)
}

// StreamClosed implements router.StreamObserver.
func (s *StreamMetrics) StreamClosed(endpoint, _ string, d time.Duration, code int) {
	h := s.handles(endpoint)
	h.open.Dec(1)
	h.duration.Update(int64(d))
	if code != 0 {
		h.closeCounter(code).Inc(1)
	}
}

// StreamMessage implements router.StreamObserver.
func (s *StreamMetrics) StreamMessage(endpoint string, toClient bool, n int) {
	h := s.handles(endpoint)
	if toClient {
		h.messagesTo.Inc(1)
		h.bytesTo.Inc(int64(n))

		return
	}
	h.messagesFrom.Inc(1)
	h.bytesFrom.Inc(int64(n))
}

func (h *streamHandles) closeCounter(code int) metrics.Counter {
	if c, ok := h.codes.Load(code); ok {
		if counter, ok := c.(metrics.Counter); ok {
			return counter
		}
	}
	c, _ := h.codes.LoadOrStore(code, h.rm.Counter("stream", h.endpoint, "close", strconv.Itoa(code)))
	counter, _ := c.(metrics.Counter) // only counters are stored

	return counter
}
