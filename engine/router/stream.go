// SPDX-License-Identifier: Apache-2.0

package router

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/9edang/inkgate/internal/constant"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/proxy"
)

// StreamNamespace is the extra_config key that turns an endpoint into a
// streaming one:
//
//	"extra_config": { "router/stream": { "mode": "event" } }
//
// Modes:
//
//	"event"  (default) Server-Sent Events and other long-lived or chunked
//	         responses. The backend must use "encoding": "no-op" so its body is
//	         passed through untouched. The body is flushed to the client after
//	         every chunk.
//	"socket" WebSocket. The gateway upgrades the client, opens a WebSocket to
//	         the backend (host and load balancing as for any backend) and
//	         relays messages, ping, pong and close in both directions.
//	         A request that is not a WebSocket upgrade gets 426.
//
// In both modes the endpoint "timeout" bounds only the wait for the backend
// (response headers, or the WebSocket handshake); once the stream is open it
// lives until the backend ends it or the client leaves. The server write
// deadline is lifted for the response. Use "concurrent_calls": 1 and a single
// backend: other combinations cancel or need the whole body.
//
// Options:
//
//	"enabled":          false turns the section off without removing it
//	"idle_timeout":     socket only, e.g. "60s": close when nothing moves for that long
//	"max_message_size": socket only, bytes; larger messages close the connection
const StreamNamespace = "inkgate/engine/router/stream"

// Streaming modes of the router/stream section.
const (
	StreamModeEvent  = "event"
	StreamModeSocket = "socket"
)

type streamConfig struct {
	mode           string // "" when the endpoint does not stream
	idleTimeout    time.Duration
	maxMessageSize int64
	maxConnections int // open streams or sockets of the endpoint at once; 0 means no limit
}

// parseStream reads the router/stream section of the endpoint. An invalid
// section is an error, so a typo fails at startup and not silently at runtime.
func parseStream(cfg *config.EndpointConfig) (streamConfig, error) {
	v, ok := cfg.ExtraConfig[StreamNamespace]
	if !ok {
		return streamConfig{}, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		m = map[string]any{}
	}
	if enabled, ok := m["enabled"].(bool); ok && !enabled {
		return streamConfig{}, nil
	}

	sc := streamConfig{mode: StreamModeEvent}
	if raw, ok := m["mode"]; ok {
		mode, _ := raw.(string)
		switch mode {
		case StreamModeEvent, StreamModeSocket:
			sc.mode = mode
		default:
			return streamConfig{}, fmt.Errorf("endpoint %s: router/stream mode must be %q or %q, got %v",
				cfg.Endpoint, StreamModeEvent, StreamModeSocket, raw)
		}
	}
	if raw, ok := m["idle_timeout"]; ok {
		str, _ := raw.(string)
		d, err := time.ParseDuration(str)
		if err != nil || d < 0 {
			return streamConfig{}, fmt.Errorf("endpoint %s: router/stream idle_timeout must be a duration like \"60s\", got %v", cfg.Endpoint, raw)
		}
		sc.idleTimeout = d
	}
	if raw, ok := m["max_message_size"]; ok {
		n, ok := config.Float64(raw)
		if i, isInt := raw.(int); isInt {
			n, ok = float64(i), true
		}
		if !ok || n < 0 {
			return streamConfig{}, fmt.Errorf("endpoint %s: router/stream max_message_size must be a number of bytes, got %v", cfg.Endpoint, raw)
		}
		sc.maxMessageSize = int64(n)
	}

	if raw, ok := m["max_connections"]; ok {
		n, ok := config.Float64(raw)
		if !ok || n < 0 || n != float64(int(n)) {
			return streamConfig{}, fmt.Errorf("endpoint %s: router/stream max_connections must be a whole number, got %v", cfg.Endpoint, raw)
		}
		sc.maxConnections = int(n)
	}

	return sc, nil
}

// connectionLimit counts the open streams of one endpoint. Taking a slot never waits: a client over the limit is
// refused at once, before the backend is called.
type connectionLimit chan struct{}

func newConnectionLimit(n int) connectionLimit {
	if n <= 0 {
		return nil // no limit
	}

	return make(connectionLimit, n)
}

// acquire takes a slot and reports whether it got one; a nil limit always does.
func (l connectionLimit) acquire() bool {
	if l == nil {
		return true
	}
	select {
	case l <- struct{}{}:
		return true
	default:
		return false
	}
}

func (l connectionLimit) release() {
	if l != nil {
		<-l
	}
}

// streamBufferSize is the size of the chunk read from the backend before each flush.
const streamBufferSize = 4 * 1024

// streamRender copies the backend body to the client, flushing after every
// chunk so events reach the client as soon as the backend sends them. It
// reports whether it handled the response: a response without a body to
// stream is left to the regular render.
func streamRender(w http.ResponseWriter, response *proxy.Response, onChunk func(n int)) bool {
	if response == nil || response.Io == nil {
		return false
	}

	h := w.Header()
	for k, vs := range response.Metadata.Headers {
		for _, v := range vs {
			h.Add(k, v)
		}
	}
	if strings.HasPrefix(strings.ToLower(h.Get(constant.HeaderContentType)), "text/event-stream") {
		if h.Get(constant.HeaderCacheControl) == "" {
			h.Set(constant.HeaderCacheControl, "no-cache")
		}
		// stop reverse proxies in front (nginx) from buffering the stream
		h.Set("X-Accel-Buffering", "no")
	}

	status := response.Metadata.StatusCode
	if status == 0 {
		status = http.StatusOK
	}

	rc := http.NewResponseController(w)
	// the server WriteTimeout would cut a stream that outlives it
	if err := rc.SetWriteDeadline(time.Time{}); err != nil && !errors.Is(err, http.ErrNotSupported) {
		return false
	}

	w.WriteHeader(status)
	flush := func() { _ = rc.Flush() }
	flush() // send the headers now: the client sees the stream is open

	buf := make([]byte, streamBufferSize)
	for {
		n, err := response.Io.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return true
			}
			flush()
			onChunk(n)
		}
		if err != nil {
			// io.EOF is the normal end; any other error means the backend or the
			// client's context ended the stream, and the headers are already sent
			return true
		}
	}
}
