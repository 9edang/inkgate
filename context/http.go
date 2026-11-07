package context

import (
	"net"
	"net/http"
	"strings"

	"github.com/9edang/inkgate/internal/json"

	"github.com/google/uuid"
)

// Namespace is the service-level extra_config key of the flow context. Alias: "context".
const Namespace = "inkgate/context"

// DefaultRequestIDHeader carries the request ID to clients and, when enabled, to backends.
const DefaultRequestIDHeader = "X-Request-Id"

// maxRequestIDLen caps client-provided IDs, so they can't bloat logs and headers.
const maxRequestIDLen = 128

// Options are the flow context settings, from extra_config["inkgate/context"].
type Options struct {
	// RequestIDHeader is the header read from clients and written to responses
	// (default X-Request-Id).
	RequestIDHeader string `json:"request_id_header"`
	// TrustRequestID reuses a valid request ID sent by the client instead of
	// generating one. Default false: a client can otherwise choose the IDs that end up in the logs.
	TrustRequestID bool `json:"trust_request_id"`
	// PropagateRequestID sends the request ID to every HTTP backend. Default true
	// (when read with ParseOptions; a zero Options value has it off).
	PropagateRequestID bool `json:"propagate_request_id"`
}

// ParseOptions reads the options from the service extra_config (config.ExtraConfig).
func ParseOptions(e map[string]any) Options {
	opts := Options{PropagateRequestID: true} // only the keys present in the config override these
	if v, ok := e[Namespace]; ok && v != nil {
		if b, err := json.Marshal(v); err == nil {
			_ = json.Unmarshal(b, &opts) // a malformed block keeps the defaults
		}
	}
	if opts.RequestIDHeader == "" {
		opts.RequestIDHeader = DefaultRequestIDHeader
	}

	return opts
}

func (o Options) trust() bool { return o.TrustRequestID }

// NewFromRequest creates the flow context of an HTTP request.
func NewFromRequest(r *http.Request, opts Options) *Flow {
	id := ""
	if opts.trust() {
		id = validID(r.Header.Get(opts.RequestIDHeader))
	}

	return New(r.Context(), Source{
		Kind:      KindHTTP,
		RequestID: id,
		Options:   opts,
		Client: Client{
			Method: r.Method,
			Path:   r.URL.Path,
			Host:   r.Host,
			IP:     remoteIP(r),
		},
	})
}

// Middleware installs a new flow context as the root of every request's
// context and writes the request ID header on the response.
func Middleware(opts Options) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := NewFromRequest(r, opts)
			w.Header().Set(opts.RequestIDHeader, c.RequestID())
			next.ServeHTTP(w, r.WithContext(c))
		})
	}
}

// newID returns a UUID version 7: it starts with the time, so IDs sort by arrival and index well in logs.
// NewV7 fails only when the random source does; a random v4 is then still a valid ID.
func newID() string {
	if id, err := uuid.NewV7(); err == nil {
		return id.String()
	}

	return uuid.NewString()
}

// validID accepts printable ASCII without spaces, up to maxRequestIDLen; anything
// else is replaced by a generated ID.
func validID(id string) string {
	if id == "" || len(id) > maxRequestIDLen {
		return ""
	}
	for i := range len(id) {
		if id[i] <= ' ' || id[i] > '~' {
			return ""
		}
	}

	return id
}

func remoteIP(r *http.Request) string {
	if ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil {
		return ip
	}

	return r.RemoteAddr
}
