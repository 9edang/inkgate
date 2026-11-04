// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"fmt"
	"github.com/9edang/inkgate/context"
	"io"
	"net/url"
	"path"
)

func main() {}

var ModifierRegisterer = registerer("inkgate-request-modifier-example")

var logger Logger = nil
var ctx context.Context = context.Background()

type registerer string

func (r registerer) RegisterModifiers(f func(
	name string,
	modifierFactory func(map[string]any) func(any) (any, error),
	appliesToRequest bool,
	appliesToResponse bool,
)) {
	f(string(r)+"-request", r.requestModifierFactory, true, false)
	f(string(r)+"-response", r.reqsponseModifierFactory, false, true)
}

func (registerer) RegisterLogger(in any) {
	l, ok := in.(Logger)
	if !ok {
		return
	}
	logger = l
	logger.Debug(fmt.Sprintf("[PLUGIN: %s] Logger loaded", ModifierRegisterer))
}

func (registerer) RegisterContext(c context.Context) {
	ctx = c
	logger.Debug(fmt.Sprintf("[PLUGIN: %s] Context loaded", ModifierRegisterer))
}

func (registerer) requestModifierFactory(_ map[string]any) func(any) (any, error) {
	// check the config
	// return the modifier

	// Graceful shutdown of any service or connection managed by the plugin
	go func() {
		<-ctx.Done()
		logger.Debug("Shuting down the service")
	}()

	if logger == nil {
		fmt.Println("request modifier loaded without logger")
		return func(input any) (any, error) {
			req, ok := input.(RequestWrapper)
			if !ok {
				return nil, unkownTypeErr
			}

			return modifier(req), nil
		}
	}

	logger.Debug(fmt.Sprintf("[PLUGIN: %s] Request modifier injected", ModifierRegisterer))
	return func(input any) (any, error) {
		req, ok := input.(RequestWrapper)
		if !ok {
			return nil, unkownTypeErr
		}

		r := modifier(req)

		requestCtx := req.Context()
		logger.Debug("context key:", requestCtx.Value("myCtxKey"))
		logger.Debug("params:", r.params)
		logger.Debug("headers:", r.headers)
		logger.Debug("method:", r.method)
		logger.Debug("url:", r.url)
		logger.Debug("query:", r.query)
		logger.Debug("path:", r.path)

		return r, nil
	}
}

func (registerer) reqsponseModifierFactory(_ map[string]any) func(any) (any, error) {
	// check the cfg. If the modifier requires some configuration,
	// it should be under the name of the plugin.
	// ex: if this modifier required some A and B config params
	/*
	   "extra_config":{
	       "plugin/req-resp-modifier":{
	           "name":["inkgate-debugger"],
	           "inkgate-debugger":{
	               "A":"foo",
	               "B":42
	           }
	       }
	   }
	*/

	go func() {
		<-ctx.Done()
		logger.Debug("Shuting down the service")
	}()

	// return the modifier
	if logger == nil {
		fmt.Println("response modifier loaded without logger")
		return func(input any) (any, error) {
			resp, ok := input.(ResponseWrapper)
			if !ok {
				return nil, unkownTypeErr
			}

			fmt.Println("data:", resp.Data())
			fmt.Println("is complete:", resp.IsComplete())
			fmt.Println("headers:", resp.Headers())
			fmt.Println("status code:", resp.StatusCode())

			return input, nil
		}
	}

	logger.Debug(fmt.Sprintf("[PLUGIN: %s] Response modifier injected", ModifierRegisterer))
	return func(input any) (any, error) {
		resp, ok := input.(ResponseWrapper)
		if !ok {
			return nil, unkownTypeErr
		}

		if req, ok := resp.Request().(RequestWrapper); ok {
			for k, v := range req.Headers() {
				logger.Debug(fmt.Sprintf("Header %s value: %s", k, v[0]))
			}
		}

		respCtx := resp.Context()
		logger.Debug("context key:", respCtx.Value("myCtxKey"))
		logger.Debug("data:", resp.Data())
		logger.Debug("is complete:", resp.IsComplete())
		logger.Debug("headers:", resp.Headers())
		logger.Debug("status code:", resp.StatusCode())

		req, ok := resp.Request().(RequestWrapper)
		if ok {
			logger.Debug("original headers:", req.Headers())
		}

		return resp, nil
	}
}

func modifier(req RequestWrapper) requestWrapper {
	return requestWrapper{
		params:  req.Params(),
		headers: req.Headers(),
		body:    req.Body(),
		method:  req.Method(),
		url:     req.URL(),
		query:   req.Query(),
		path:    path.Join(req.Path(), "/fooo"),
	}
}

var unkownTypeErr = errors.New("unknown request type")

type ResponseWrapper interface {
	Context() context.Context
	Request() any
	Data() map[string]any
	IsComplete() bool
	Headers() map[string][]string
	StatusCode() int
}

type RequestWrapper interface {
	Context() context.Context
	Params() map[string]string
	Headers() map[string][]string
	Body() io.ReadCloser
	Method() string
	URL() *url.URL
	Query() url.Values
	Path() string
}

type requestWrapper struct {
	ctx     context.Context
	method  string
	url     *url.URL
	query   url.Values
	path    string
	body    io.ReadCloser
	params  map[string]string
	headers map[string][]string
}

func (r requestWrapper) Context() context.Context     { return r.ctx }
func (r requestWrapper) Method() string               { return r.method }
func (r requestWrapper) URL() *url.URL                { return r.url }
func (r requestWrapper) Query() url.Values            { return r.query }
func (r requestWrapper) Path() string                 { return r.path }
func (r requestWrapper) Body() io.ReadCloser          { return r.body }
func (r requestWrapper) Params() map[string]string    { return r.params }
func (r requestWrapper) Headers() map[string][]string { return r.headers }

type Logger interface {
	Debug(v ...any)
	Info(v ...any)
	Warning(v ...any)
	Error(v ...any)
	Critical(v ...any)
	Fatal(v ...any)
}
