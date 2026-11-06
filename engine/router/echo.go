// SPDX-License-Identifier: Apache-2.0

package router

import (
	"io"
	"net/http"

	"github.com/9edang/inkgate/internal/constant"

	"github.com/9edang/inkgate/internal/json"
)

type echoResponse struct {
	Uri         string              `json:"req_uri"`
	UriDetails  map[string]string   `json:"req_uri_details"`
	Method      string              `json:"req_method"`
	Querystring map[string][]string `json:"req_querystring"`
	Body        string              `json:"req_body"`
	Headers     map[string][]string `json:"req_headers"`
}

// EchoHandler creates a dummy handler function, useful for quick integration tests.
func EchoHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body string
		if r.Body != nil {
			tmp, _ := io.ReadAll(r.Body)
			// #nosec G104 -- closing is best effort, the error is not actionable
			r.Body.Close()
			body = string(tmp)
		}
		resp, err := json.Marshal(echoResponse{
			Uri: r.RequestURI,
			UriDetails: map[string]string{
				"user":     r.URL.User.String(),
				"host":     r.Host,
				"path":     r.URL.Path,
				"query":    r.URL.Query().Encode(),
				"fragment": r.URL.Fragment,
			},
			Method:      r.Method,
			Querystring: r.URL.Query(),
			Body:        body,
			Headers:     r.Header,
		})
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		w.Header().Set(constant.HeaderContentType, constant.MIMEJSONUTF8)
		// #nosec G104 -- response write is best effort, the client may be gone
		w.Write(resp)
	}
}
