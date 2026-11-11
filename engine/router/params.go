package router

import (
	"maps"

	"github.com/9edang/inkgate/context"
	"net/http"
	"net/textproto"

	gorilla "github.com/gorilla/mux"
)

type extraParamsKey struct{}

// WithParams returns r carrying extra params for the request builder, for
// example JWT claims used as {{.JWT.sub}} in a backend url_pattern.
func WithParams(r *http.Request, params map[string]string) *http.Request {
	if len(params) == 0 {
		return r
	}
	merged := map[string]string{}
	if prev, ok := r.Context().Value(extraParamsKey{}).(map[string]string); ok {
		maps.Copy(merged, prev)
	}
	maps.Copy(merged, params)
	return r.WithContext(context.WithValue(r.Context(), extraParamsKey{}, merged))
}

// PathParamExtractor returns the path variables gorilla matched (/users/{id}),
// plus any params added with WithParams. Keys are capitalized the same way in
// every inkgate build ("id" -> "Id"), so url_pattern placeholders work as written.
func PathParamExtractor(r *http.Request) map[string]string {
	vars := gorilla.Vars(r)
	params := make(map[string]string, len(vars))
	for name, value := range vars {
		params[textproto.CanonicalMIMEHeaderKey(name[:1])+name[1:]] = value
	}
	if extra, ok := r.Context().Value(extraParamsKey{}).(map[string]string); ok {
		maps.Copy(params, extra)
	}
	return params
}

// NewRequestWithPathParams is the RequestBuilder used by inkgate: like
// NewRequest, but it also forwards the path params.
var NewRequestWithPathParams = NewRequestBuilder(PathParamExtractor)

// ClientIP returns the best guess of the client IP: X-Forwarded-For, then
// X-Real-Ip, then the connection address.
func ClientIP(r *http.Request) string { return clientIP(r) }
