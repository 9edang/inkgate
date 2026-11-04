// Package context is the inkgate end-to-end flow context: one value created
// when a request enters the gateway and carried by every layer until the
// backend services have answered.
//
// The package also re-exports the standard library context API (std.go):
// Context is an alias of the standard context.Context, so the whole project
// imports this package instead of "context", with no alias:
//
//	import "github.com/9edang/inkgate/context"
//
//	func step(ctx context.Context, ...) {
//		flow := context.From(ctx) // the flow context of this request
//		flow.RequestID()
//		flow.Set("tenant", "acme")
//		ctx, cancel := context.WithTimeout(ctx, time.Second) // still carries flow
//	}
//
// *Flow implements context.Context. It is installed as the root of the
// request's context, so every context a layer derives from it (timeouts,
// cancels, OpenTelemetry spans) still finds it with From.
//
// The data is shared by all layers and all concurrent backend calls of the
// request, and every method is safe for concurrent use. Methods are also safe
// on a nil *Flow, so code that runs outside a request (tests, tools) does
// not need to check: reads return zero values and writes are dropped.
package context

import (
	std "context"
	"maps"
	"sync"
	"time"
)

// Kind says what started the context.
const (
	KindHTTP  = "http"  // a client request
	KindAsync = "async" // a message consumed by an async agent
)

type ctxKey struct{}

// PluginKey lets code that does not import this package — Go .so plugins in
// particular — reach the flow through any request context:
//
//	type flow interface {
//		RequestID() string
//		Get(key string) (any, bool)
//		Set(key string, value any)
//	}
//	if f, ok := ctx.Value("inkgate.flow").(flow); ok { ... }
//
// It is a plain string on purpose: a plugin can use it without sharing any
// type with the gateway, so it keeps working across gateway rebuilds.
const PluginKey = "inkgate.flow"

// Flow is the end-to-end flow context of one request: it flows with the request from
// the entry stage until the backends have answered. Create it with New or
// NewFromRequest; read it back with From.
type Flow struct {
	std.Context
	data *data
}

type data struct {
	mu sync.RWMutex

	requestID string
	kind      string
	source    string
	startedAt time.Time
	client    Client

	endpoint string
	method   string
	params   map[string]string
	claims   map[string]any
	backends []BackendCall
	values   map[string]any

	propagate bool
	header    string
}

// Client describes who sent an HTTP request.
type Client struct {
	Method string
	Path   string
	Host   string
	IP     string
}

// BackendCall is the record of one call to one backend.
type BackendCall struct {
	Backend  string // the backend url_pattern
	Method   string
	Hosts    []string
	Status   int  // backend status code when known, 0 otherwise
	Complete bool // the backend answered completely
	Duration time.Duration
	Err      string
}

// Source describes the origin of a new Flow.
type Source struct {
	Kind      string // KindHTTP or KindAsync
	Name      string // async agent name; empty for HTTP
	RequestID string // empty: a new ID is generated
	Client    Client
	Options   Options
}

// New returns a Flow rooted at parent. It is usually created once per
// request by the entry stage, or per message by an async agent.
func New(parent std.Context, src Source) *Flow {
	if parent == nil {
		parent = std.Background()
	}
	id := src.RequestID
	if id == "" {
		id = newID()
	}
	header := src.Options.RequestIDHeader
	if header == "" {
		header = DefaultRequestIDHeader
	}
	return &Flow{
		Context: parent,
		data: &data{
			requestID: id,
			kind:      src.Kind,
			source:    src.Name,
			startedAt: time.Now(),
			client:    src.Client,
			propagate: src.Options.PropagateRequestID,
			header:    header,
		},
	}
}

// From returns the flow context carried by ctx, or nil when there is none.
// The result is always safe to use (see the package documentation).
func From(ctx std.Context) *Flow {
	if ctx == nil {
		return nil
	}
	if c, ok := ctx.(*Flow); ok {
		return c
	}
	c, _ := ctx.Value(ctxKey{}).(*Flow)
	return c
}

// Value returns the Flow itself for its own key and for PluginKey, and
// delegates every other key to the parent, as any context.Context does.
func (c *Flow) Value(key any) any {
	switch k := key.(type) {
	case ctxKey:
		return c
	case string:
		if k == PluginKey {
			return c
		}
	}
	return c.Context.Value(key)
}

// RequestID is the ID of the request, taken from the client (when trusted) or generated.
func (c *Flow) RequestID() string {
	if c == nil {
		return ""
	}
	return c.data.requestID
}

// Kind is KindHTTP or KindAsync.
func (c *Flow) Kind() string {
	if c == nil {
		return ""
	}
	return c.data.kind
}

// Source is the async agent name for KindAsync contexts.
func (c *Flow) Source() string {
	if c == nil {
		return ""
	}
	return c.data.source
}

// StartedAt is when the request entered the gateway.
func (c *Flow) StartedAt() time.Time {
	if c == nil {
		return time.Time{}
	}
	return c.data.startedAt
}

// Elapsed is the time since the request entered the gateway.
func (c *Flow) Elapsed() time.Duration {
	if c == nil {
		return 0
	}
	return time.Since(c.data.startedAt)
}

// Client describes the HTTP client (zero for async contexts).
func (c *Flow) Client() Client {
	if c == nil {
		return Client{}
	}
	return c.data.client
}

// RequestIDHeader is the header that carries the request ID.
func (c *Flow) RequestIDHeader() string {
	if c == nil {
		return ""
	}
	return c.data.header
}

// PropagatesRequestID reports whether the request ID is sent to backends.
func (c *Flow) PropagatesRequestID() bool {
	if c == nil {
		return false
	}
	return c.data.propagate
}

// Endpoint returns the endpoint pattern and method the router matched.
func (c *Flow) Endpoint() (pattern, method string) {
	if c == nil {
		return "", ""
	}
	c.data.mu.RLock()
	defer c.data.mu.RUnlock()
	return c.data.endpoint, c.data.method
}

// SetEndpoint records the matched endpoint (set by the endpoint handler).
func (c *Flow) SetEndpoint(pattern, method string) {
	if c == nil {
		return
	}
	c.data.mu.Lock()
	c.data.endpoint, c.data.method = pattern, method
	c.data.mu.Unlock()
}

// Params returns a copy of the path params (keys as sent to backends: "Id").
func (c *Flow) Params() map[string]string {
	if c == nil {
		return nil
	}
	c.data.mu.RLock()
	defer c.data.mu.RUnlock()
	return maps.Clone(c.data.params)
}

// SetParams records the path params (set by the endpoint handler).
func (c *Flow) SetParams(params map[string]string) {
	if c == nil {
		return
	}
	c.data.mu.Lock()
	c.data.params = maps.Clone(params)
	c.data.mu.Unlock()
}

// Claims returns a copy of the validated JWT claims, or nil.
func (c *Flow) Claims() map[string]any {
	if c == nil {
		return nil
	}
	c.data.mu.RLock()
	defer c.data.mu.RUnlock()
	return maps.Clone(c.data.claims)
}

// SetClaims records the validated JWT claims (set by the JWT validator).
func (c *Flow) SetClaims(claims map[string]any) {
	if c == nil {
		return
	}
	c.data.mu.Lock()
	c.data.claims = maps.Clone(claims)
	c.data.mu.Unlock()
}

// AddBackendCall records one finished backend call.
func (c *Flow) AddBackendCall(call BackendCall) {
	if c == nil {
		return
	}
	c.data.mu.Lock()
	c.data.backends = append(c.data.backends, call)
	c.data.mu.Unlock()
}

// BackendCalls returns the backend calls recorded so far, in finishing order.
func (c *Flow) BackendCalls() []BackendCall {
	if c == nil {
		return nil
	}
	c.data.mu.RLock()
	defer c.data.mu.RUnlock()
	out := make([]BackendCall, len(c.data.backends))
	copy(out, c.data.backends)
	return out
}

// Set stores a custom value under key, for later layers to read with Get.
// Prefer namespaced keys ("lua.score", "tenant") to avoid collisions.
func (c *Flow) Set(key string, value any) {
	if c == nil {
		return
	}
	c.data.mu.Lock()
	if c.data.values == nil {
		c.data.values = map[string]any{}
	}
	c.data.values[key] = value
	c.data.mu.Unlock()
}

// Get returns the custom value stored under key.
func (c *Flow) Get(key string) (any, bool) {
	if c == nil {
		return nil, false
	}
	c.data.mu.RLock()
	defer c.data.mu.RUnlock()
	v, ok := c.data.values[key]
	return v, ok
}

// Delete removes a custom value.
func (c *Flow) Delete(key string) {
	if c == nil {
		return
	}
	c.data.mu.Lock()
	delete(c.data.values, key)
	c.data.mu.Unlock()
}
