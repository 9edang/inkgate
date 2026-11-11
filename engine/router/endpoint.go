// SPDX-License-Identifier: Apache-2.0

package router

import (
	"errors"

	"github.com/9edang/inkgate/internal/constant"

	"github.com/9edang/inkgate/context"

	"fmt"
	"net"
	"net/http"
	"net/textproto"
	"slices"
	"strings"
	"time"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/core"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/transport/http/server"
	"github.com/9edang/inkgate/engine/transport/http/socket"
)

const requestParamsAsterisk string = "*"

// HandlerFactory creates a handler function that adapts the router with the injected proxy.
type HandlerFactory func(*config.EndpointConfig, proxy.Proxy) http.HandlerFunc

// EndpointHandler is a HandlerFactory that adapts the router with the injected proxy
// and the default RequestBuilder.
var EndpointHandler = CustomEndpointHandler(NewRequest)

// CustomEndpointHandler returns a HandlerFactory with the received RequestBuilder using the default ToHTTPError function.
func CustomEndpointHandler(rb RequestBuilder) HandlerFactory {
	return CustomEndpointHandlerWithHTTPError(rb, server.DefaultToHTTPError)
}

// CustomEndpointHandlerWithHTTPError returns a HandlerFactory with the received RequestBuilder.
//
//nolint:cyclop,gocognit,gocyclo // one long handler, split it when it is next changed
func CustomEndpointHandlerWithHTTPError(rb RequestBuilder, errF server.ToHTTPError) HandlerFactory {
	return func(configuration *config.EndpointConfig, prxy proxy.Proxy) http.HandlerFunc {
		cacheControlHeaderValue := fmt.Sprintf("public, max-age=%d", int(configuration.CacheTTL.Seconds()))
		isCacheEnabled := configuration.CacheTTL.Seconds() != 0
		render := getRender(configuration)
		sc, err := parseStream(configuration)
		if err != nil {
			// fail at startup, like an invalid route pattern
			panic(err)
		}
		stream := sc.mode != ""
		socketMode := sc.mode == StreamModeSocket
		limit := newConnectionLimit(sc.maxConnections)
		observer := currentStreamObserver()
		endpoint := configuration.Endpoint

		headersToSend := configuration.HeadersToPass
		if len(headersToSend) == 0 {
			headersToSend = server.HeadersToSend
		}
		method := strings.ToTitle(configuration.Method)

		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(core.InkgateHeaderName, core.InkgateHeaderValue)
			if r.Method != method {
				w.Header().Set(server.CompleteResponseHeaderName, server.HeaderIncompleteResponseValue)
				w.WriteHeader(http.StatusMethodNotAllowed)

				return
			}

			flow := context.From(r.Context())
			flow.SetEndpoint(configuration.Endpoint, method)

			if socketMode && !socket.IsWebSocket(r) {
				w.Header().Set("Upgrade", "websocket")
				w.Header().Set(server.CompleteResponseHeaderName, server.HeaderIncompleteResponseValue)
				w.WriteHeader(http.StatusUpgradeRequired)

				return
			}

			closeCode := 0 // the socket relay reports it (OnClose), on this goroutine
			if stream {
				// the handler runs as long as the stream or the socket: the slot is held for its whole life
				if !limit.acquire() {
					observer.StreamRefused(endpoint, sc.mode)
					w.Header().Set("Retry-After", "1")
					w.Header().Set(server.CompleteResponseHeaderName, server.HeaderIncompleteResponseValue)
					w.WriteHeader(http.StatusServiceUnavailable)

					return
				}
				defer limit.release()
				// the shutdown waits for this stream, and tells it to end (see below)
				defer server.DrainFrom(r.Context()).Track()()
				opened := time.Now()
				observer.StreamOpened(endpoint, sc.mode)
				defer func() { observer.StreamClosed(endpoint, sc.mode, time.Since(opened), closeCode) }()
			}

			var requestCtx context.Context
			var cancel context.CancelFunc
			var headerTimer *time.Timer
			var upgrade *socket.Upgrade
			if stream {
				// the timeout only covers the wait for the backend response; the
				// stream itself ends with the backend or when the client leaves
				requestCtx, cancel = context.WithCancel(r.Context())
				headerTimer = time.AfterFunc(configuration.Timeout, cancel)
				// on shutdown an SSE stream ends: cancelling the backend request ends the copy, and the client
				// gets the end of a complete response
				drain := server.DrainFrom(r.Context())
				defer context.AfterFunc(drain.Context(), cancel)()
				if socketMode {
					upgrade = &socket.Upgrade{
						W:                w,
						R:                r,
						HandshakeTimeout: configuration.Timeout,
						IdleTimeout:      sc.idleTimeout,
						MaxMessageSize:   sc.maxMessageSize,
						Established:      func() { headerTimer.Stop() },
						GoingAway:        drain.Done(),
						OnMessage:        func(toClient bool, n int) { observer.StreamMessage(endpoint, toClient, n) },
						OnClose:          func(code int) { closeCode = code },
					}
					requestCtx = socket.WithUpgrade(requestCtx, upgrade)
				}
			} else {
				requestCtx, cancel = context.WithTimeout(r.Context(), configuration.Timeout)
			}

			proxyReq := rb(r, configuration.QueryString, headersToSend)
			flow.SetParams(proxyReq.Params)
			response, err := prxy(requestCtx, proxyReq)
			if headerTimer != nil {
				headerTimer.Stop()
			}
			if upgrade != nil && upgrade.Handled() {
				// the connection was upgraded (and is closed by now) or the
				// backend refusal was already sent
				cancel()
				return
			}

			select {
			case <-requestCtx.Done():
				if err == nil {
					err = server.ErrInternalError
				}
			default:
			}

			if response != nil && (len(response.Data) > 0 || response.Raw != nil) {
				if response.IsComplete {
					w.Header().Set(server.CompleteResponseHeaderName, server.HeaderCompleteResponseValue)
					if isCacheEnabled {
						w.Header().Set(constant.HeaderCacheControl, cacheControlHeaderValue)
					}
				} else {
					w.Header().Set(server.CompleteResponseHeaderName, server.HeaderIncompleteResponseValue)
				}

				for k, vs := range response.Metadata.Headers {
					for _, v := range vs {
						w.Header().Add(k, v)
					}
				}
			} else {
				w.Header().Set(server.CompleteResponseHeaderName, server.HeaderIncompleteResponseValue)
				if err != nil {
					writeError(w, err, errF)
					cancel()

					return
				}
			}

			if stream && !socketMode && err == nil && response != nil && response.IsComplete {
				w.Header().Set(server.CompleteResponseHeaderName, server.HeaderCompleteResponseValue)
			}
			if stream && !socketMode && err == nil && streamRender(w, response, func(n int) { observer.StreamMessage(endpoint, true, n) }, sc.keepAlive) {
				cancel()
				return
			}

			if configuration.OutputEncoding == NEGOTIATE {
				negotiatedRender(w, r, response)
			} else {
				render(w, response)
			}
			cancel()
		}
	}
}

// RequestBuilder is a function that creates a proxy.Request from the received http request.
type RequestBuilder func(r *http.Request, queryString, headersToSend []string) *proxy.Request

// ParamExtractor is a function that extracts query params from the requested uri.
type ParamExtractor func(r *http.Request) map[string]string

// NoopParamExtractor is a No Op ParamExtractor (returns an empty map of params).
func NoopParamExtractor(_ *http.Request) map[string]string { return map[string]string{} }

// NewRequest is a RequestBuilder that creates a proxy request from the received http request without
// processing the uri params.
var NewRequest = NewRequestBuilder(NoopParamExtractor)

// NewRequestBuilder gets a RequestBuilder with the received ParamExtractor as a query param
// extraction mechanism.
func NewRequestBuilder(paramExtractor ParamExtractor) RequestBuilder {
	return func(r *http.Request, queryString, headersToSend []string) *proxy.Request {
		params := paramExtractor(r)
		headers := make(map[string][]string, 3+len(headersToSend))

		for _, k := range headersToSend {
			if k == requestParamsAsterisk {
				headers = r.Header

				break
			}

			if h, ok := r.Header[textproto.CanonicalMIMEHeaderKey(k)]; ok {
				headers[k] = h
			}
		}

		headers[constant.HeaderXForwardedFor] = []string{clientIP(r)}
		headers["X-Forwarded-Host"] = []string{r.Host}
		// if User-Agent is not forwarded using headersToSend, we set
		// the Inkgate router User Agent value
		if _, ok := headers[constant.HeaderUserAgent]; !ok {
			headers[constant.HeaderUserAgent] = server.UserAgentHeaderValue
		} else {
			headers["X-Forwarded-Via"] = server.UserAgentHeaderValue
		}

		query := make(map[string][]string, len(queryString))
		queryValues := r.URL.Query()
		for i := range queryString {
			if queryString[i] == requestParamsAsterisk {
				query = queryValues

				break
			}

			if v, ok := queryValues[queryString[i]]; ok && len(v) > 0 {
				query[queryString[i]] = v
			}
		}

		return &proxy.Request{
			Path:    r.URL.Path,
			Method:  r.Method,
			Query:   query,
			Body:    r.Body,
			Params:  params,
			Headers: headers,
		}
	}
}

type responseError interface {
	error
	StatusCode() int
}

// clientIP implements a best effort algorithm to return the real client IP, it parses
// X-Real-IP and X-Forwarded-For in order to work properly with reverse-proxies such us: nginx or haproxy.
// Use X-Forwarded-For before X-Real-Ip as nginx uses X-Real-Ip with the proxy's IP.
func clientIP(r *http.Request) string {
	clientIP := r.Header.Get(constant.HeaderXForwardedFor)
	clientIP = strings.TrimSpace(strings.Split(clientIP, ",")[0])
	if clientIP == "" {
		clientIP = strings.TrimSpace(r.Header.Get("X-Real-IP"))
	}
	if clientIP != "" {
		return clientIP
	}

	if addr := r.Header.Get("X-Appengine-Remote-Addr"); addr != "" {
		return addr
	}

	if ip, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr)); err == nil {
		return ip
	}

	return ""
}

type encodedResponseError interface {
	responseError
	Encoding() string
}

type headerResponseError interface {
	responseError
	Headers() map[string][]string
}

// writeError writes the status of err (its own, or errF's mapping), the headers
// it carries and, with return_error_msg, its message.
func writeError(w http.ResponseWriter, err error, errF server.ToHTTPError) {
	if withHeaders, ok := errors.AsType[headerResponseError](err); ok {
		for k, vs := range withHeaders.Headers() {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}
	}
	status := errF(err)
	if withStatus, ok := errors.AsType[responseError](err); ok {
		status = withStatus.StatusCode()
	}
	// a body cut by max_body_bytes while it was forwarded: the client sent too much, the backend did not fail
	if bodyTooLarge(err) {
		status = http.StatusRequestEntityTooLarge
	}
	if !ReturnErrorMsg {
		w.WriteHeader(status)
		return
	}
	contentType := "text/plain; charset=utf-8"
	if withEncoding, ok := errors.AsType[encodedResponseError](err); ok && withEncoding.Encoding() != "" {
		contentType = withEncoding.Encoding()
	}
	w.Header().Set(constant.HeaderContentType, contentType)
	w.WriteHeader(status)
	// #nosec G104 -- response write is best effort, the client may be gone
	w.Write([]byte(err.Error()))
}

// bodyTooLarge reports whether err comes from a body cut by max_body_bytes, also inside the error of a merged
// endpoint (which lists the error of each backend without unwrapping them).
func bodyTooLarge(err error) bool {
	if _, ok := errors.AsType[*http.MaxBytesError](err); ok {
		return true
	}
	if multi, ok := err.(interface{ Errors() []error }); ok {
		return slices.ContainsFunc(multi.Errors(), bodyTooLarge)
	}

	return false
}
