// SPDX-License-Identifier: Apache-2.0

package router

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
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
//	"allowed_origins":  socket only, e.g. ["https://app.example.com", "https://*.example.com"]: an upgrade with
//	                    another Origin gets 403 before the backend is dialed; one without Origin (not a browser) passes.
//	                    Without the option only the same origin is allowed (the Origin host is the Host of the request);
//	                    ["*"] allows every origin
//	"subprotocols":     socket only, e.g. ["graphql-transport-ws"]: only these reach the backend; a client that offers
//	                    some and none of these gets 400
//	"max_messages_per_second": socket only: messages and pings a client may send per second; over it the socket is
//	                    closed with 1008 (policy violation). "message_burst" (default: one second of messages) more at once
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
	maxConnections int               // open streams or sockets of the endpoint at once; 0 means no limit
	keepAlive      time.Duration     // an SSE stream silent that long gets a comment line; 0 means never
	origins        originPolicy      // socket only: who may open a socket; the zero value allows the same origin only
	subprotocols   subprotocolPolicy // socket only: the subprotocols passed to the backend; nil passes all
	messageRate    float64           // socket only: client messages per second; 0 means no limit
	messageBurst   int               // socket only: messages above the rate a client may send at once
}

// streamKeys are the options of the router/stream section. A key that is not here is a typo: it would be ignored.
var streamKeys = map[string]bool{
	"enabled": true, "mode": true, "idle_timeout": true, "max_message_size": true, "keep_alive": true,
	"max_connections": true, "allowed_origins": true, "subprotocols": true,
	"max_messages_per_second": true, "message_burst": true,
}

// ValidateStream checks the router/stream section of an endpoint, the same way the router does when it builds the
// endpoint, so `inkgate check` and `inkgate run` refuse a bad section before anything is served. It finds the
// section under its alias too, so it can run before Normalize.
func ValidateStream(cfg *config.EndpointConfig) error {
	_, err := parseStream(cfg)

	return err
}

// parseStream reads the router/stream section of the endpoint. A section that is not an object, an option it does not
// know or an invalid value is an error, so a typo fails at startup and not silently at runtime.
func parseStream(cfg *config.EndpointConfig) (streamConfig, error) {
	v, ok := cfg.ExtraConfig.Get(StreamNamespace)
	if !ok {
		return streamConfig{}, nil
	}
	m, ok := v.(map[string]any)
	if !ok {
		return streamConfig{}, fmt.Errorf("endpoint %s: router/stream must be an object like {\"mode\": \"event\"}, got %v", cfg.Endpoint, v)
	}
	for _, k := range sortedKeys(m) {
		if !streamKeys[k] {
			return streamConfig{}, fmt.Errorf("endpoint %s: router/stream has an unknown option %q%s", cfg.Endpoint, k,
				didYouMean(k, sortedKeys(streamKeys)))
		}
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

	if raw, ok := m["keep_alive"]; ok {
		str, _ := raw.(string)
		d, err := time.ParseDuration(str)
		if err != nil || d <= 0 {
			return streamConfig{}, fmt.Errorf("endpoint %s: router/stream keep_alive must be a duration like \"15s\", got %v", cfg.Endpoint, raw)
		}
		sc.keepAlive = d
	}
	if raw, ok := m["max_connections"]; ok {
		n, ok := config.Float64(raw)
		if !ok || n < 0 || n != float64(int(n)) {
			return streamConfig{}, fmt.Errorf("endpoint %s: router/stream max_connections must be a whole number, got %v", cfg.Endpoint, raw)
		}
		sc.maxConnections = int(n)
	}
	if raw, ok := m["allowed_origins"]; ok {
		list, ok := stringList(raw)
		if !ok {
			return streamConfig{}, fmt.Errorf("endpoint %s: router/stream allowed_origins must be a list of origins, got %v", cfg.Endpoint, raw)
		}
		p, err := parseOriginPolicy(list)
		if err != nil {
			return streamConfig{}, fmt.Errorf("endpoint %s: router/stream allowed_origins: %w", cfg.Endpoint, err)
		}
		sc.origins = p
	}
	if raw, ok := m["subprotocols"]; ok {
		list, ok := stringList(raw)
		if !ok {
			return streamConfig{}, fmt.Errorf("endpoint %s: router/stream subprotocols must be a list of names, got %v", cfg.Endpoint, raw)
		}
		sc.subprotocols = subprotocolPolicy(list)
	}
	if sc.mode != StreamModeSocket && (sc.origins.enabled() || sc.subprotocols != nil) {
		// an SSE request is a plain fetch, which CORS already covers: a check here would only look like protection
		return streamConfig{}, fmt.Errorf("endpoint %s: router/stream allowed_origins and subprotocols are for mode %q", cfg.Endpoint, StreamModeSocket)
	}
	if err := parseMessageRate(m, &sc); err != nil {
		return streamConfig{}, fmt.Errorf("endpoint %s: router/stream %w", cfg.Endpoint, err)
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
func streamRender(w http.ResponseWriter, response *proxy.Response, onChunk func(n int), keepAlive time.Duration) bool {
	if response == nil || response.Io == nil {
		return false
	}

	eventStream := streamHeaders(w.Header(), response)

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
	if keepAlive > 0 && eventStream {
		sseCopy{w: w, body: response.Io, buf: buf, flush: flush, onChunk: onChunk, keepAlive: keepAlive}.run()

		return true
	}
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

// streamHeaders copies the backend headers and, for an SSE stream, adds the ones that keep proxies in front from
// caching or buffering it. It reports whether the stream is SSE.
func streamHeaders(h http.Header, response *proxy.Response) bool {
	for k, vs := range response.Metadata.Headers {
		for _, v := range vs {
			h.Add(k, v)
		}
	}
	eventStream := strings.HasPrefix(strings.ToLower(h.Get(constant.HeaderContentType)), "text/event-stream")
	if eventStream {
		if h.Get(constant.HeaderCacheControl) == "" {
			h.Set(constant.HeaderCacheControl, "no-cache")
		}
		// stop reverse proxies in front (nginx) from buffering the stream
		h.Set("X-Accel-Buffering", "no")
	}

	return eventStream
}

// keepAliveComment is an SSE comment: every client ignores it, and it keeps proxies in front from closing a silent
// connection.
var keepAliveComment = []byte(": keep-alive\n\n")

// sseCopy is an SSE stream being copied to the client with keep-alive comments.
type sseCopy struct {
	w         http.ResponseWriter
	body      io.Reader
	buf       []byte
	flush     func()
	onChunk   func(int)
	keepAlive time.Duration
}

// chunk is one read from the backend.
type chunk struct {
	n   int
	err error
}

// run copies the stream like streamRender, and writes a keep-alive comment when the backend sent nothing for
// keepAlive. A comment goes only between events: a pause in the middle of one waits for its end, since a line written
// inside an event would break it. The backend is read on another goroutine so the silence can be timed; all writes to
// the client stay on this one.
func (c sseCopy) run() {
	next := make(chan struct{})
	stop := make(chan struct{})
	defer close(stop)
	chunks := readChunks(c.body, c.buf, next, stop)

	timer := time.NewTimer(c.keepAlive)
	defer timer.Stop()
	var boundary eventBoundary
	for {
		select {
		case ch := <-chunks:
			if ch.n > 0 {
				if _, err := c.w.Write(c.buf[:ch.n]); err != nil {
					return
				}
				c.flush()
				c.onChunk(ch.n)
				boundary.wrote(c.buf[:ch.n])
			}
			if ch.err != nil {
				return // EOF, or the backend or the client ended the stream
			}
			timer.Reset(c.keepAlive)
			next <- struct{}{}
		case <-timer.C:
			if boundary.between() {
				if _, err := c.w.Write(keepAliveComment); err != nil {
					return
				}
				c.flush()
			}
			timer.Reset(c.keepAlive)
		}
	}
}

// readChunks reads body into buf on its own goroutine. buf is not read into again before next: the receiver writes
// it out first. It stops at the first error or when stop is closed.
func readChunks(body io.Reader, buf []byte, next, stop <-chan struct{}) <-chan chunk {
	chunks := make(chan chunk)
	go func() {
		for {
			n, err := body.Read(buf)
			select {
			case chunks <- chunk{n, err}:
			case <-stop:
				return
			}
			if err != nil {
				return
			}
			select {
			case <-next:
			case <-stop:
				return
			}
		}
	}()

	return chunks
}

// eventBoundary tracks whether the bytes written so far end an SSE event (a blank line), from their last 4 bytes.
type eventBoundary struct {
	tail    [4]byte
	n       int
	started bool
}

func (e *eventBoundary) wrote(b []byte) {
	e.started = true
	if len(b) >= len(e.tail) {
		e.n = copy(e.tail[:], b[len(b)-len(e.tail):])
		return
	}
	var t [4]byte
	keep := min(e.n, len(e.tail)-len(b))
	n := copy(t[:], e.tail[e.n-keep:e.n])
	n += copy(t[n:], b)
	e.tail, e.n = t, n
}

// between reports whether a comment can go now: before the first byte, or right after the end of an event.
func (e *eventBoundary) between() bool {
	if !e.started {
		return true
	}
	b := e.tail[:e.n]

	return bytes.HasSuffix(b, []byte("\n\n")) || bytes.HasSuffix(b, []byte("\r\n\r\n")) || bytes.HasSuffix(b, []byte("\r\r"))
}

// parseMessageRate reads max_messages_per_second and message_burst (socket only). The burst defaults to one second of
// messages, at least 1.
func parseMessageRate(m map[string]any, sc *streamConfig) error {
	rawRate, hasRate := m["max_messages_per_second"]
	rawBurst, hasBurst := m["message_burst"]
	if !hasRate {
		if hasBurst {
			return errors.New("message_burst needs max_messages_per_second")
		}

		return nil
	}
	if sc.mode != StreamModeSocket {
		return fmt.Errorf("max_messages_per_second is for mode %q", StreamModeSocket)
	}
	r, ok := config.Float64(rawRate)
	if !ok || r <= 0 {
		return fmt.Errorf("max_messages_per_second must be a number above 0, got %v", rawRate)
	}
	sc.messageRate = r
	sc.messageBurst = max(1, int(math.Ceil(r)))
	if hasBurst {
		b, ok := config.Float64(rawBurst)
		if !ok || b < 1 || b != float64(int(b)) {
			return fmt.Errorf("message_burst must be a whole number from 1, got %v", rawBurst)
		}
		sc.messageBurst = int(b)
	}

	return nil
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	return keys
}

func didYouMean(name string, candidates []string) string {
	if c := config.Suggest(name, candidates); c != "" {
		return fmt.Sprintf(" (did you mean %q?)", c)
	}

	return ""
}
