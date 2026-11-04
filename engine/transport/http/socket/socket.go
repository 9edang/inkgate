// SPDX-License-Identifier: Apache-2.0

/*
Package socket proxies WebSocket connections between a client and a backend.

The endpoint handler (engine/router/mux) puts an Upgrade in the request
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
	"github.com/9edang/inkgate/context"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"

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
	// Established is called once, when both connections are open and before
	// the relay starts. The endpoint uses it to stop its handshake timeout.
	Established func()

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
// gorilla refuses them in the request headers
var skipHeaders = map[string]bool{
	"Upgrade":                  true,
	"Connection":               true,
	"Sec-Websocket-Key":        true,
	"Sec-Websocket-Version":    true,
	"Sec-Websocket-Extensions": true,
	"Sec-Websocket-Protocol":   true, // sent through Dialer.Subprotocols
	"Sec-Websocket-Accept":     true,
	"Content-Length":           true,
	"Host":                     true,
}

// ErrNoHijack is returned when the client connection cannot be upgraded, for
// example on HTTP/2.
var ErrNoHijack = errors.New("the client connection does not support upgrades")

// Proxy opens the WebSocket to the backend URL of req, upgrades the client
// and relays messages until one side closes the connection.
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

	dialer := websocket.Dialer{
		Subprotocols: websocket.Subprotocols(u.R),
	}
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

	upgrader := websocket.Upgrader{
		// answer with the subprotocol the backend picked, and no other
		Subprotocols: []string{backend.Subprotocol()},
		// the backend already validated the Origin; the gateway enforces its own
		// policy with the security/cors and bot checks of the request stage
		CheckOrigin: func(*http.Request) bool { return true },
	}
	if backend.Subprotocol() == "" {
		upgrader.Subprotocols = nil
	}

	hw := hijackable(u.W)
	if hw == nil {
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
		backend.Close()
		return nil, err
	}
	if u.Established != nil {
		u.Established()
	}

	relay(client, backend, u.IdleTimeout, u.MaxMessageSize)
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

// relay copies messages both ways until one side ends, then closes both.
func relay(client, backend *websocket.Conn, idle time.Duration, maxMessage int64) {
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
			client.Close()
			backend.Close()
		})
	}

	// ping and pong are end-to-end: each side's keep-alive reaches the other
	forwardControl := func(from, to *websocket.Conn) {
		from.SetPingHandler(func(data string) error {
			touch()
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

	var wg sync.WaitGroup
	wg.Add(2)
	pipe := func(from, to *websocket.Conn) {
		defer wg.Done()
		for {
			typ, r, err := from.NextReader()
			if err != nil {
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
			w, err := to.NextWriter(typ)
			if err != nil {
				closeAll()
				return
			}
			if _, err := io.Copy(w, r); err != nil {
				w.Close()
				closeAll()
				return
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
}

// propagateClose tells the other side how this one ended: a close frame keeps
// its code and reason, anything else (connection lost) is an abnormal closure.
func propagateClose(to *websocket.Conn, err error) {
	var ce *websocket.CloseError
	var msg []byte
	if errors.As(err, &ce) {
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
