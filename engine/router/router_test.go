//go:build !race

// SPDX-License-Identifier: Apache-2.0

package router

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/9edang/inkgate/context"
	"io"
	"net"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/transport/http/server"
)

func TestDefaultFactory_ok(t *testing.T) {
	buff := bytes.NewBuffer(make([]byte, 1024))
	logger, err := logging.NewLogger("ERROR", buff, "pref")
	if err != nil {
		t.Error("building the logger:", err.Error())
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		time.Sleep(5 * time.Millisecond)
	}()

	r := DefaultFactory(noopProxyFactory(map[string]any{"supu": "tupu"}), logger).NewWithContext(ctx)
	expectedBody := "{\"supu\":\"tupu\"}"

	port := freePort(t)
	serviceCfg := config.ServiceConfig{
		Port: port,
		Endpoints: []*config.EndpointConfig{
			{
				Endpoint: "/get",
				Method:   "GET",
				Timeout:  10,
				Backend: []*config.Backend{
					{},
				},
			},
			{
				Endpoint: "/get",
				Method:   "POST",
				Timeout:  10,
				Backend: []*config.Backend{
					{},
				},
			},
			{
				Endpoint: "/post",
				Method:   "Post",
				Timeout:  10,
				Backend: []*config.Backend{
					{},
				},
			},
			{
				Endpoint: "/put",
				Method:   "put",
				Timeout:  10,
				Backend: []*config.Backend{
					{},
				},
			},
			{
				Endpoint: "/patch",
				Method:   "PATCH",
				Timeout:  10,
				Backend: []*config.Backend{
					{},
				},
			},
			{
				Endpoint: "/delete",
				Method:   "DELETE",
				Timeout:  10,
				Backend: []*config.Backend{
					{},
				},
			},
		},
	}

	go func() { r.Run(serviceCfg) }()

	waitListening(t, port)

	for _, endpoint := range serviceCfg.Endpoints {
		req, _ := http.NewRequest(strings.ToTitle(endpoint.Method), fmt.Sprintf("http://127.0.0.1:%d%s", port, endpoint.Endpoint), http.NoBody)
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error("Making the request:", err.Error())
			return
		}
		defer resp.Body.Close()

		body, ioerr := io.ReadAll(resp.Body)
		if ioerr != nil {
			t.Error("Reading the response:", ioerr.Error())
			return
		}
		content := string(body)
		if resp.Header.Get("Cache-Control") != "" {
			t.Error(endpoint.Endpoint, "Cache-Control error:", resp.Header.Get("Cache-Control"))
		}
		if resp.Header.Get(server.CompleteResponseHeaderName) != server.HeaderCompleteResponseValue {
			t.Error(server.CompleteResponseHeaderName, "error:", resp.Header.Get(server.CompleteResponseHeaderName))
		}
		if resp.Header.Get("Content-Type") != "application/json; charset=utf-8" {
			t.Error(endpoint.Endpoint, "Content-Type error:", resp.Header.Get("Content-Type"))
		}
		if resp.Header.Get("X-Inkgate") != "Version undefined" {
			t.Error(endpoint.Endpoint, "X-Inkgate error:", resp.Header.Get("X-Inkgate"))
		}
		if resp.StatusCode != http.StatusOK {
			t.Error(endpoint.Endpoint, "Unexpected status code:", resp.StatusCode)
		}
		if content != expectedBody {
			t.Error(endpoint.Endpoint, "Unexpected body:", content, "expected:", expectedBody)
		}
	}
}

func TestDefaultFactory_ko(t *testing.T) {
	buff := bytes.NewBuffer(make([]byte, 1024))
	logger, err := logging.NewLogger("ERROR", buff, "pref")
	if err != nil {
		t.Error("building the logger:", err.Error())
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		time.Sleep(5 * time.Millisecond)
	}()

	r := NewFactory(Config{
		Engine:         DefaultEngine(),
		Middlewares:    []HandlerMiddleware{identityMiddleware{}},
		HandlerFactory: EndpointHandler,
		ProxyFactory:   noopProxyFactory(map[string]any{"supu": "tupu"}),
		Logger:         logger,
		RunServer:      server.RunServer,
	}).NewWithContext(ctx)

	port := freePort(t)
	serviceCfg := config.ServiceConfig{
		Debug: true,
		Port:  port,
		Endpoints: []*config.EndpointConfig{
			{
				Endpoint: "/ignored",
				Method:   "GETTT",
				Backend: []*config.Backend{
					{},
				},
			},
			{
				Endpoint: "/empty",
				Method:   "GETTT",
				Backend:  []*config.Backend{},
			},
		},
	}

	go func() { r.Run(serviceCfg) }()

	waitListening(t, port)

	for _, subject := range [][]string{
		{"GET", "ignored"},
		{"GET", "empty"},
		{"PUT", "also-ignored"},
	} {
		req, _ := http.NewRequest(subject[0], fmt.Sprintf("http://127.0.0.1:%d/%s", port, subject[1]), http.NoBody)
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		checkResponseIs404(t, req)
	}
}

func TestDefaultFactory_proxyFactoryCrash(t *testing.T) {
	buff := bytes.NewBuffer(make([]byte, 1024))
	logger, err := logging.NewLogger("ERROR", buff, "pref")
	if err != nil {
		t.Error("building the logger:", err.Error())
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
		time.Sleep(5 * time.Millisecond)
	}()

	r := DefaultFactory(erroredProxyFactory{fmt.Errorf("%s", "crash!!!")}, logger).NewWithContext(ctx)

	port := freePort(t)
	serviceCfg := config.ServiceConfig{
		Debug: true,
		Port:  port,
		Endpoints: []*config.EndpointConfig{
			{
				Endpoint: "/ignored",
				Method:   "GET",
				Timeout:  10,
				Backend: []*config.Backend{
					{},
				},
			},
		},
	}

	go func() { r.Run(serviceCfg) }()

	waitListening(t, port)

	for _, subject := range [][]string{{"GET", "ignored"}, {"PUT", "also-ignored"}} {
		req, _ := http.NewRequest(subject[0], fmt.Sprintf("http://127.0.0.1:%d/%s", port, subject[1]), http.NoBody)
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		checkResponseIs404(t, req)
	}
}

func TestRunServer_ko(t *testing.T) {
	buff := new(bytes.Buffer)
	logger, err := logging.NewLogger("DEBUG", buff, "")
	if err != nil {
		t.Error("building the logger:", err.Error())
		return
	}

	errorMsg := "runServer error"
	runServerFunc := func(_ context.Context, _ config.ServiceConfig, _ http.Handler) error {
		return errors.New(errorMsg)
	}

	pf := noopProxyFactory(map[string]any{"supu": "tupu"})
	r := NewFactory(
		Config{
			Engine:         DefaultEngine(),
			Middlewares:    []HandlerMiddleware{},
			HandlerFactory: EndpointHandler,
			ProxyFactory:   pf,
			Logger:         logger,
			DebugPattern:   DefaultDebugPattern,
			RunServer:      runServerFunc,
		},
	).New()

	serviceCfg := config.ServiceConfig{}
	r.Run(serviceCfg)
	re := regexp.MustCompile(errorMsg)
	if !re.MatchString(buff.String()) {
		t.Errorf("the logger doesn't contain the expected msg: %s", buff.Bytes())
	}
}

func checkResponseIs404(t *testing.T, req *http.Request) {
	expectedBody := "404 page not found\n"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Error("Making the request:", err.Error())
		return
	}
	defer resp.Body.Close()
	body, ioerr := io.ReadAll(resp.Body)
	if ioerr != nil {
		t.Error("Reading the response:", ioerr.Error())
		return
	}
	content := string(body)
	if resp.Header.Get("Cache-Control") != "" {
		t.Error("Cache-Control error:", resp.Header.Get("Cache-Control"))
	}
	if resp.Header.Get(server.CompleteResponseHeaderName) != server.HeaderIncompleteResponseValue {
		t.Error(req.URL.String(), server.CompleteResponseHeaderName, "error:", resp.Header.Get(server.CompleteResponseHeaderName))
	}
	if resp.Header.Get("Content-Type") != "text/plain; charset=utf-8" {
		t.Error("Content-Type error:", resp.Header.Get("Content-Type"))
	}
	if resp.Header.Get("X-Inkgate") != "" {
		t.Error("X-Inkgate error:", resp.Header.Get("X-Inkgate"))
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Error("Unexpected status code:", resp.StatusCode)
	}
	if content != expectedBody {
		t.Error("Unexpected body:", content, "expected:", expectedBody)
	}
}

type noopProxyFactory map[string]any

func (n noopProxyFactory) New(_ *config.EndpointConfig) (proxy.Proxy, error) {
	return func(_ context.Context, _ *proxy.Request) (*proxy.Response, error) {
		return &proxy.Response{
			IsComplete: true,
			Data:       n,
		}, nil
	}, nil
}

type erroredProxyFactory struct {
	Error error
}

func (e erroredProxyFactory) New(_ *config.EndpointConfig) (proxy.Proxy, error) {
	return proxy.NoopProxy, e.Error
}

type identityMiddleware struct{}

func (identityMiddleware) Handler(h http.Handler) http.Handler {
	return h
}

// waitListening waits until the server accepts connections on port, instead of sleeping a fixed time.
func waitListening(t *testing.T, port int) {
	t.Helper()
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if c, err := net.Dial("tcp", addr); err == nil {
			c.Close()
			return
		}
	}
	t.Fatalf("nothing listening on %s", addr)
}
