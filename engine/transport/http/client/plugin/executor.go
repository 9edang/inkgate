// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"fmt"
	"github.com/9edang/inkgate/context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/transport/http/client"
)

const (
	Namespace = "inkgate/engine/transport/http/client/plugin"
	// middlewareNamespace is the internal namespace of the register for chainable plugins
	middlewareNamespace = Namespace + "/middleware"
)

func HTTPRequestExecutor(
	logger logging.Logger,
	next func(*config.Backend) client.HTTPRequestExecutor,
) func(*config.Backend) client.HTTPRequestExecutor {
	return HTTPRequestExecutorWithContext(context.Background(), logger, next)
}

func HTTPRequestExecutorWithContext(
	ctx context.Context,
	logger logging.Logger,
	next func(*config.Backend) client.HTTPRequestExecutor,
) func(*config.Backend) client.HTTPRequestExecutor {
	return func(cfg *config.Backend) client.HTTPRequestExecutor {
		logPrefix := fmt.Sprintf("[BACKEND: %s %s -> %s]", cfg.ParentEndpointMethod, cfg.ParentEndpoint, cfg.URLPattern)
		v, ok := cfg.ExtraConfig[Namespace]
		if !ok {
			return next(cfg)
		}
		extra, ok := v.(map[string]any)
		if !ok {
			logger.Debug(logPrefix, "["+Namespace+"]", "Wrong extra config type for backend")
			return next(cfg)
		}

		// load plugin(s)
		r, ok := clientRegister.Get(Namespace)
		mr, mok := clientRegister.Get(middlewareNamespace)
		if !ok && !mok {
			logger.Debug(logPrefix, "No plugins registered for the module")
			return next(cfg)
		}

		fifo := pluginNames(extra["name"])
		if len(fifo) == 0 {
			logger.Debug(logPrefix, "No name defined in the extra config for", cfg.URLPattern)
			return next(cfg)
		}

		// The regular backend call closes the chain; the plugins wrap it so the
		// first name of the list is the outermost one.
		handler := http.Handler(backendHandler(func() client.HTTPRequestExecutor { return next(cfg) }))
		injected := 0
		for _, name := range slices.Backward(fifo) {

			if mok {
				if rawMf, ok := mr.Get(name); ok {
					mf, ok := rawMf.(nextHandlerFunc)
					if !ok {
						logger.Warning(logPrefix, "Wrong plugin handler type:", name)
						continue
					}
					wrapped, err := mf(ctx, extra, handler)
					if err != nil {
						logger.Warning(logPrefix, "Error getting the plugin handler:", err.Error())
						continue
					}
					logger.Debug(logPrefix, "Injecting plugin", name)
					handler = wrapped
					injected++
					continue
				}
			}

			var rawHf any
			if ok {
				rawHf, ok = r.Get(name)
			}
			if !ok {
				logger.Debug(logPrefix, "No plugin registered as", name)
				continue
			}

			hf, ok := rawHf.(func(context.Context, map[string]any) (http.Handler, error))
			if !ok {
				logger.Warning(logPrefix, "Wrong plugin handler type:", name)
				continue
			}

			// a plain client plugin replaces the backend call and cannot call
			// the next stage, so everything after it in the list is dropped
			h, err := hf(ctx, extra)
			if err != nil {
				logger.Warning(logPrefix, "Error getting the plugin handler:", err.Error())
				continue
			}
			if injected > 0 {
				logger.Warning(logPrefix, "Plugin", name, "replaces the backend call: the plugins after it are not executed")
			}
			logger.Debug(logPrefix, "Injecting plugin", name)
			handler = h
			injected = 1
		}

		if injected == 0 {
			return next(cfg)
		}

		return func(ctx context.Context, req *http.Request) (*http.Response, error) {
			var backendErr error
			ctx = context.WithValue(ctx, backendErrKey{}, &backendErr)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req.WithContext(ctx))
			if backendErr != nil {
				return nil, backendErr
			}
			return w.Result(), nil
		}
	}
}

// pluginNames returns the plugin names set in the "name" key of the extra
// config: a single string or a list of them.
func pluginNames(v any) []string {
	switch n := v.(type) {
	case string:
		return []string{n}
	case []string:
		return n
	case []any:
		names := make([]string, 0, len(n))
		for _, x := range n {
			if name, ok := x.(string); ok {
				names = append(names, name)
			}
		}
		return names
	}
	return nil
}

type backendErrKey struct{}

// backendHandler exposes the regular backend call as the last http.Handler of
// the plugin chain. A transport error cannot be written to the response, so it
// is handed to the executor through the request context and returned as is.
//
// The executor is built on the first call, so a plugin that replaces the
// backend call never causes the regular client to be created.
func backendHandler(factory func() client.HTTPRequestExecutor) http.Handler {
	exec := sync.OnceValue(factory)
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		resp, err := exec()(req.Context(), req)
		if err != nil {
			if slot, ok := req.Context().Value(backendErrKey{}).(*error); ok {
				*slot = err
			}
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		for k, vv := range resp.Header {
			for _, v := range vv {
				w.Header().Add(k, v)
			}
		}
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	})
}
