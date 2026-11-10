package router

import (
	"sync/atomic"
	"time"
)

// StreamObserver is told about the life of the streams and sockets of streaming endpoints, for metrics. Its methods
// are called on the request path, once per message for sockets and per chunk for event streams, so they must be cheap
// and safe for concurrent use.
type StreamObserver interface {
	// StreamOpened: a stream or socket of the endpoint started (mode is "event" or "socket").
	StreamOpened(endpoint, mode string)
	// StreamRefused: a request over max_connections got 503.
	StreamRefused(endpoint, mode string)
	// StreamClosed: it ended after d. For a socket, code is the close code (1000 normal, 1001 going away, 1006 dropped
	// without a close frame...); for an event stream it is 0.
	StreamClosed(endpoint, mode string, d time.Duration, code int)
	// StreamMessage: a message (socket) or chunk (event stream) of n bytes went to the client, or came from it.
	StreamMessage(endpoint string, toClient bool, n int)
}

type noopObserver struct{}

func (noopObserver) StreamOpened(string, string)                     {}
func (noopObserver) StreamRefused(string, string)                    {}
func (noopObserver) StreamClosed(string, string, time.Duration, int) {}
func (noopObserver) StreamMessage(string, bool, int)                 {}

// observerBox holds the observer: atomic.Value would panic on a second observer of another concrete type.
type observerBox struct{ StreamObserver }

var streamObserver atomic.Pointer[observerBox]

func init() { streamObserver.Store(&observerBox{noopObserver{}}) }

// RegisterStreamObserver sets the observer of every streaming endpoint built afterwards. The metrics layer registers
// one at startup; without it, nothing is observed.
func RegisterStreamObserver(o StreamObserver) {
	if o == nil {
		o = noopObserver{}
	}
	streamObserver.Store(&observerBox{o})
}

func currentStreamObserver() StreamObserver {
	return streamObserver.Load().StreamObserver
}
