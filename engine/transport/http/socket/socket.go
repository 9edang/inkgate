// SPDX-License-Identifier: Apache-2.0

/*
Package socket proxies WebSocket connections between a client and a backend.

The endpoint handler (engine/router) puts an Upgrade in the request
context when the endpoint is configured with "router/stream" mode "socket".
The backend step (BackendFactory) finds it after the pipeline has picked the
backend host: it opens the WebSocket to the backend first, and only when the
backend accepted it upgrades the client connection, negotiating the same
subprotocol. Then it relays messages both ways, with their type, and forwards
ping, pong and close frames (with their close code and reason), so both ends
see each other's keep-alive and shutdown as if they were connected directly.

A backend that refuses the upgrade has its answer (status, headers, body)
sent to the client as it is.
*/
package socket

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/9edang/inkgate/internal/constant"

	"github.com/9edang/inkgate/context"

	"github.com/gorilla/websocket"
	"golang.org/x/time/rate"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/proxy"
)

// Upgrade is what the endpoint handler hands to the backend step: the client
// connection that may be upgraded and the settings of the proxy.
type Upgrade struct {
	W http.ResponseWriter
	R *http.Request
	// HandshakeTimeout bounds the connection to the backend and its answer to
	// the upgrade; zero means only the request context bounds it. A deadline is
	// needed because the dialer does not stop on a cancelled context while it
	// waits for the backend answer.
	HandshakeTimeout time.Duration
	// IdleTimeout closes the connection when no message, ping or pong moves in
	// either direction for that long; zero disables it.
	IdleTimeout time.Duration
	// MaxMessageSize limits the size of a message read from either side; zero
	// means no limit.
	MaxMessageSize int64
	// MessageRate limits the messages (and pings) a client sends, per second, with MessageBurst more at once; over it
	// the connection is closed with 1008 (policy violation). Zero means no limit; a burst below 1 means 1.
	MessageRate  float64
	MessageBurst int
	// Subprotocols are offered to the backend, in the client's order of preference; the endpoint keeps there the
	// ones of the client it allows. Nil offers none.
	Subprotocols []string
	// Established is called once, when both connections are open and before
	// the relay starts. The endpoint uses it to stop its handshake timeout.
	Established func()
	// GoingAway, when closed, ends the relay with a close frame 1001 (going away) to both sides: the server is
	// shutting down. Nil means never.
	GoingAway <-chan struct{}
	// OnMessage, when set, is called for every message relayed, with its direction and size, from the relay
	// goroutines: it must be safe for concurrent use.
	OnMessage func(toClient bool, n int)
	// OnClose, when set, is called once when the relay ended, from the goroutine that called Proxy, with the close
	// code: the one of the first close frame, 1001 when the server shut down, 1006 when a side dropped without one.
	OnClose func(code int)

	handled atomic.Bool
}

// Handled tells if the proxy already answered the client, by upgrading the
// connection or with an error. The endpoint handler must not write anything then.
func (u *Upgrade) Handled() bool { return u.handled.Load() }

type ctxKey struct{}

// WithUpgrade returns a context carrying u.
func WithUpgrade(ctx context.Context, u *Upgrade) context.Context {
	return context.WithValue(ctx, ctxKey{}, u)
}

// From returns the Upgrade of the context, or nil for a regular request.
func From(ctx context.Context) *Upgrade {
	u, _ := ctx.Value(ctxKey{}).(*Upgrade)
	return u
}

// Subprotocols returns the subprotocols the client offers, in its order of preference.
func Subprotocols(r *http.Request) []string { return websocket.Subprotocols(r) }

// IsWebSocket tells if r asks for a WebSocket upgrade.
func IsWebSocket(r *http.Request) bool { return websocket.IsWebSocketUpgrade(r) }

// BackendFactory returns a factory whose proxies handle the request as a
// WebSocket when the context carries an Upgrade and call the next proxy otherwise.
func BackendFactory(next proxy.BackendFactory) proxy.BackendFactory {
	return func(remote *config.Backend) proxy.Proxy {
		n := next(remote)
		return func(ctx context.Context, r *proxy.Request) (*proxy.Response, error) {
			if u := From(ctx); u != nil {
				return Proxy(ctx, u, r)
			}

			return n(ctx, r)
		}
	}
}

// not forwarded to the backend: the handshake of each side is its own, or
// gorilla refuses them in the request headers.
var skipHeaders = map[string]bool{
	"Upgrade":                    true,
	"Connection":                 true,
	"Sec-Websocket-Key":          true,
	"Sec-Websocket-Version":      true,
	"Sec-Websocket-Extensions":   true,
	"Sec-Websocket-Protocol":     true, // sent through Dialer.Subprotocols
	"Sec-Websocket-Accept":       true,
	constant.HeaderContentLength: true,
	"Host":                       true,
}

// ErrNoHijack is returned when the client connection cannot be upgraded, for
// example on HTTP/2.
var ErrNoHijack = errors.New("the client connection does not support upgrades")

// Proxy opens the WebSocket to the backend URL of req, upgrades the client
// and relays messages until one side closes the connection.
//
//nolint:cyclop,gocyclo // the handshake steps stay in one place to read in order
func Proxy(ctx context.Context, u *Upgrade, req *proxy.Request) (*proxy.Response, error) {
	if req.URL == nil {
		return nil, errors.New("socket: the request has no backend URL")
	}

	header := make(http.Header, len(req.Headers)+1)
	for k, vs := range req.Headers {
		if skipHeaders[http.CanonicalHeaderKey(k)] {
			continue
		}
		header[k] = append([]string(nil), vs...)
	}
	// the backend decides on the Origin like it would from the browser
	if origin := u.R.Header.Get("Origin"); origin != "" {
		header.Set("Origin", origin)
	}

	dialer := websocket.Dialer{Subprotocols: u.Subprotocols}
	dialCtx := ctx
	if u.HandshakeTimeout > 0 {
		var cancel context.CancelFunc
		dialCtx, cancel = context.WithTimeout(ctx, u.HandshakeTimeout)
		defer cancel()
	}
	backend, resp, err := dialer.DialContext(dialCtx, wsURL(req.URL), header)
	if err != nil {
		if errors.Is(err, websocket.ErrBadHandshake) && resp != nil {
			relayRefusal(u, resp)
			return &proxy.Response{}, nil
		}

		return nil, err
	}

	// gorilla takes the backend's choice as it is: one that was not offered would reach the client, which must fail
	// the connection on it (RFC 6455 4.1), or slip past the endpoint's subprotocols list
	if p := backend.Subprotocol(); p != "" && !slices.Contains(u.Subprotocols, p) {
		// #nosec G104 -- closing is best effort, the error is not actionable
		backend.Close()
		return nil, fmt.Errorf("socket: the backend chose the subprotocol %q, which was not offered", p)
	}

	upgrader := websocket.Upgrader{
		// answer with the subprotocol the backend picked, and no other
		Subprotocols: []string{backend.Subprotocol()},
		// the endpoint already checked the Origin before dialing the backend: against its allowed_origins, or, without
		// that list, against the Host of the request (CORS does not apply to WebSocket). Checking again here would
		// refuse the origins the list allows.
		CheckOrigin: func(*http.Request) bool { return true },
	}
	if backend.Subprotocol() == "" {
		upgrader.Subprotocols = nil
	}

	hw := hijackable(u.W)
	if hw == nil {
		// #nosec G104 -- closing is best effort, the error is not actionable
		backend.Close()
		u.handled.Store(true)
		http.Error(u.W, ErrNoHijack.Error(), http.StatusNotImplemented)

		return nil, ErrNoHijack
	}

	// the headers the gateway already set (X-Inkgate, request id...) go with the
	// 101 answer; the handshake ones belong to the upgrader
	answer := u.W.Header().Clone()
	for k := range answer {
		if skipHeaders[http.CanonicalHeaderKey(k)] {
			delete(answer, k)
		}
	}
	// Upgrade writes the error answer itself when the client handshake is bad
	client, err := upgrader.Upgrade(hw, u.R, answer)
	u.handled.Store(true)
	if err != nil {
		// #nosec G104 -- closing is best effort, the error is not actionable
		backend.Close()
		return nil, err
	}
	if u.Established != nil {
		u.Established()
	}

	code := relay(client, backend, relayOptions{
		idle: u.IdleTimeout, maxMessage: u.MaxMessageSize, goingAway: u.GoingAway, onMessage: u.OnMessage,
		clientRate: newMessageLimit(u.MessageRate, u.MessageBurst),
	})
	if u.OnClose != nil {
		u.OnClose(code)
	}

	return &proxy.Response{IsComplete: true}, nil
}

// wsURL turns the backend URL into the ws/wss one the dialer needs.
func wsURL(u *url.URL) string {
	c := *u
	switch c.Scheme {
	case "https", "wss":
		c.Scheme = "wss"
	default:
		c.Scheme = "ws"
	}

	return c.String()
}

// relayRefusal sends the backend answer to a refused upgrade to the client.
func relayRefusal(u *Upgrade, resp *http.Response) {
	defer resp.Body.Close()
	u.handled.Store(true)
	for k, vs := range resp.Header {
		for _, v := range vs {
			u.W.Header().Add(k, v)
		}
	}
	u.W.WriteHeader(resp.StatusCode)
	_, _ = io.Copy(u.W, resp.Body)
}

// hijackable finds, through the response writer wrappers, the writer that can
// be hijacked: gorilla needs the http.Hijacker itself, not a controller.
func hijackable(w http.ResponseWriter) http.ResponseWriter {
	for w != nil {
		if _, ok := w.(http.Hijacker); ok {
			return w
		}
		uw, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return nil
		}
		w = uw.Unwrap()
	}

	return nil
}

const controlWait = 5 * time.Second

type relayOptions struct {
	idle       time.Duration
	maxMessage int64
	goingAway  <-chan struct{}
	onMessage  func(toClient bool, n int)
	clientRate *rate.Limiter // nil means no limit
}

// newMessageLimit returns the token bucket of one connection, or nil without a limit. It starts full: a client may send
// burst messages at once right after the upgrade (a subscription, an auth message).
func newMessageLimit(perSecond float64, burst int) *rate.Limiter {
	if perSecond <= 0 {
		return nil
	}

	return rate.NewLimiter(rate.Limit(perSecond), max(burst, 1))
}

// relay copies messages both ways until one side ends, then closes both. It returns the close code: the first close
// frame seen, 1001 when the server shut down, 1006 (abnormal) when a side dropped without one.
//
//nolint:cyclop,gocognit,gocyclo // one loop per direction, split it when it is next changed
func relay(client, backend *websocket.Conn, o relayOptions) int {
	idle, maxMessage, goingAway := o.idle, o.maxMessage, o.goingAway
	var closeCode atomic.Int64 // first one wins
	if maxMessage > 0 {
		client.SetReadLimit(maxMessage)
		backend.SetReadLimit(maxMessage)
	}

	var last atomic.Int64
	touch := func() { last.Store(time.Now().UnixNano()) }
	touch()

	var once sync.Once
	closeAll := func() {
		once.Do(func() {
			// #nosec G104 -- closing is best effort, the error is not actionable
			client.Close()
			// #nosec G104 -- closing is best effort, the error is not actionable
			backend.Close()
		})
	}

	// a client over its message rate gets 1008, and so does the backend (the reason tells it why); what the client sends
	// until it answers the close is dropped. Only touched from the client's reading goroutine.
	overLimit := false
	allowed := func() bool {
		if overLimit {
			return false
		}
		if o.clientRate == nil || o.clientRate.Allow() {
			return true
		}
		overLimit = true
		closeCode.CompareAndSwap(0, websocket.ClosePolicyViolation)
		msg := websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "message rate exceeded")
		_ = client.WriteControl(websocket.CloseMessage, msg, time.Now().Add(controlWait))
		_ = backend.WriteControl(websocket.CloseMessage, msg, time.Now().Add(controlWait))
		time.AfterFunc(controlWait, closeAll)

		return false
	}

	// ping and pong are end-to-end: each side's keep-alive reaches the other
	forwardControl := func(from, to *websocket.Conn) {
		from.SetPingHandler(func(data string) error {
			touch()
			if from == client && !allowed() {
				return nil // a ping flood is a message flood
			}
			return to.WriteControl(websocket.PingMessage, []byte(data), time.Now().Add(controlWait))
		})
		from.SetPongHandler(func(data string) error {
			touch()
			return to.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(controlWait))
		})
	}
	forwardControl(client, backend)
	forwardControl(backend, client)

	finished := make(chan struct{})
	if idle > 0 {
		go func() {
			t := time.NewTicker(idle / 4)
			defer t.Stop()
			for {
				select {
				case <-finished:
					return
				case <-t.C:
					if time.Since(time.Unix(0, last.Load())) >= idle {
						closeAll()
						return
					}
				}
			}
		}()
	}

	if goingAway != nil {
		go func() {
			select {
			case <-finished:
			case <-goingAway:
				// the server is shutting down: both sides learn it with 1001; their close answers end the pipes,
				// and the connections are dropped after controlWait whatever they do
				closeCode.CompareAndSwap(0, websocket.CloseGoingAway)
				msg := websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutting down")
				_ = client.WriteControl(websocket.CloseMessage, msg, time.Now().Add(controlWait))
				_ = backend.WriteControl(websocket.CloseMessage, msg, time.Now().Add(controlWait))
				time.AfterFunc(controlWait, closeAll)
			}
		}()
	}

	var wg sync.WaitGroup
	wg.Add(2)
	pipe := func(from, to *websocket.Conn) {
		defer wg.Done()
		toClient := to == client
		for {
			typ, r, err := from.NextReader()
			if err != nil {
				if ce, ok := errors.AsType[*websocket.CloseError](err); ok {
					closeCode.CompareAndSwap(0, int64(ce.Code))
				} else {
					closeCode.CompareAndSwap(0, websocket.CloseAbnormalClosure)
				}
				propagateClose(to, err)
				if websocket.IsCloseError(err) || websocket.IsUnexpectedCloseError(err) {
					// a close frame was forwarded: give the other side the time to
					// answer it before the connections are dropped
					time.AfterFunc(controlWait, closeAll)
				} else {
					closeAll()
				}

				return
			}
			touch()
			if !toClient && !allowed() {
				if _, err := io.Copy(io.Discard, r); err != nil {
					closeAll()
					return
				}

				continue
			}
			w, err := to.NextWriter(typ)
			if err != nil {
				closeAll()
				return
			}
			n, err := io.Copy(w, r)
			if err != nil {
				// #nosec G104 -- closing is best effort, the error is not actionable
				w.Close()
				closeAll()

				return
			}
			if o.onMessage != nil {
				o.onMessage(toClient, int(n))
			}
			if err := w.Close(); err != nil {
				closeAll()
				return
			}
		}
	}
	go pipe(client, backend)
	go pipe(backend, client)
	wg.Wait()
	close(finished)

	return int(closeCode.Load())
}

// propagateClose tells the other side how this one ended: a close frame keeps
// its code and reason, anything else (connection lost) is an abnormal closure.
func propagateClose(to *websocket.Conn, err error) {
	var msg []byte
	if ce, ok := errors.AsType[*websocket.CloseError](err); ok {
		code := ce.Code
		// codes that must not be sent on the wire (1005, 1006, 1015)
		if code == websocket.CloseNoStatusReceived || code == websocket.CloseAbnormalClosure || code == websocket.CloseTLSHandshake {
			msg = websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
		} else {
			msg = websocket.FormatCloseMessage(code, ce.Text)
		}
	} else {
		msg = websocket.FormatCloseMessage(websocket.CloseGoingAway, "")
	}
	_ = to.WriteControl(websocket.CloseMessage, msg, time.Now().Add(controlWait))
}
