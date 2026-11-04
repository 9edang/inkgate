// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"github.com/9edang/inkgate/context"
	"html"
	"net/http"
)

// HandlerRegisterer is the symbol the plugin loader will try to load. It must implement the Registerer interface
var HandlerRegisterer = registerer("inkgate-server-example")

type registerer string

var logger Logger = nil

func (registerer) RegisterLogger(v any) {
	l, ok := v.(Logger)
	if !ok {
		return
	}
	logger = l
	logger.Debug(fmt.Sprintf("[PLUGIN: %s] Logger loaded", HandlerRegisterer))
}

func (r registerer) RegisterHandlers(f func(
	name string,
	handler func(context.Context, map[string]any, http.Handler) (http.Handler, error),
)) {
	f(string(r), r.registerHandlers)
}

func (registerer) registerHandlers(_ context.Context, _ map[string]any, _ http.Handler) (http.Handler, error) {
	// check the passed configuration and initialize the plugin
	// possible config example:
	/*
	   "extra_config":{
	       "plugin/http-server":{
	           "name":["inkgate-server-example"],
	           "inkgate-server-example":{
	               "A":"foo",
	               "B":42
	           }
	       }
	   }
	*/

	if logger == nil {
		// return the actual handler wrapping or your custom logic so it can be used as a replacement for the default http handler
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			fmt.Fprintf(w, "Hello, %q", html.EscapeString(req.URL.Path))
		}), nil
	}

	// return the actual handler wrapping or your custom logic so it can be used as a replacement for the default http handler
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		fmt.Fprintf(w, "Hello, %q", html.EscapeString(req.URL.Path))
		logger.Debug("request:", html.EscapeString(req.URL.Path))
	}), nil
}

func main() {}

type Logger interface {
	Debug(v ...any)
	Info(v ...any)
	Warning(v ...any)
	Error(v ...any)
	Critical(v ...any)
	Fatal(v ...any)
}
