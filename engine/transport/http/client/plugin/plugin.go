// SPDX-License-Identifier: Apache-2.0

package plugin

import (
	"fmt"
	"github.com/9edang/inkgate/context"
	"net/http"
	"plugin"
	"strings"

	"github.com/9edang/inkgate/engine/logging"
	coreplugin "github.com/9edang/inkgate/engine/plugin"
	"github.com/9edang/inkgate/engine/register"
)

var clientRegister = register.New()

func RegisterClient(
	name string,
	handler func(context.Context, map[string]any) (http.Handler, error),
) {
	clientRegister.Register(Namespace, name, handler)
}

type Registerer interface {
	RegisterClients(func(
		name string,
		handler func(context.Context, map[string]any) (http.Handler, error),
	))
}

// nextHandlerFunc builds a client plugin that can be chained with others: it
// receives the handler of the next stage (the following plugin of the list or,
// for the last one, the regular backend HTTP call) and returns the handler that
// replaces it. It calls next.ServeHTTP to continue the chain or answers by itself.
type nextHandlerFunc func(ctx context.Context, extra map[string]any, next http.Handler) (http.Handler, error)

// registerClientNext stores a chainable plugin under the given name.
func registerClientNext(name string, handler nextHandlerFunc) {
	clientRegister.Register(middlewareNamespace, name, handler)
}

// nextRegisterer is the plugin interface that receives next. It has the
// same method name as Registerer; the loader tells them apart by signature.
// A plugin implementing only Registerer keeps working: it replaces the backend
// call and ends the chain.
type nextRegisterer interface {
	RegisterClients(func(
		name string,
		handler func(context.Context, map[string]any, http.Handler) (http.Handler, error),
	))
}

type LoggerRegisterer interface {
	RegisterLogger(any)
}

type RegisterClientFunc func(
	name string,
	handler func(context.Context, map[string]any) (http.Handler, error),
)

func Load(path, pattern string, rcf RegisterClientFunc) (int, error) {
	return LoadWithLogger(path, pattern, rcf, nil)
}

func LoadWithLogger(path, pattern string, rcf RegisterClientFunc, logger logging.Logger) (int, error) {
	plugins, err := coreplugin.Scan(path, pattern)
	if err != nil {
		return 0, err
	}
	return load(plugins, rcf, logger)
}

func load(plugins []string, rcf RegisterClientFunc, logger logging.Logger) (int, error) {
	var errors []error

	loadedPlugins := 0
	for k, pluginName := range plugins {
		if err := open(pluginName, rcf, logger); err != nil {
			errors = append(errors, fmt.Errorf("plugin #%d (%s): %s", k, pluginName, err.Error()))
			continue
		}
		loadedPlugins++
	}

	if len(errors) > 0 {
		return loadedPlugins, loaderError{errors: errors}
	}
	return loadedPlugins, nil
}

func open(pluginName string, rcf RegisterClientFunc, logger logging.Logger) (err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
		}
	}()

	var p Plugin
	p, err = pluginOpener(pluginName)
	if err != nil {
		return
	}
	var r any
	r, err = p.Lookup("ClientRegisterer")
	if err != nil {
		return
	}
	registerer, isClient := r.(Registerer)
	middlewareRegisterer, isMiddleware := r.(nextRegisterer)
	if !isClient && !isMiddleware {
		return fmt.Errorf("http-request-executor plugin loader: unknown type")
	}

	if logger != nil {
		if lr, ok := r.(LoggerRegisterer); ok {
			lr.RegisterLogger(logger)
		}
	}

	RegisterExtraComponents(r)

	if isClient {
		registerer.RegisterClients(rcf)
	}
	if isMiddleware {
		middlewareRegisterer.RegisterClients(func(
			name string,
			handler func(context.Context, map[string]any, http.Handler) (http.Handler, error),
		) {
			registerClientNext(name, handler)
		})
	}
	return
}

var RegisterExtraComponents = func(any) {}

// Plugin is the interface of the loaded plugins
type Plugin interface {
	Lookup(name string) (plugin.Symbol, error)
}

// pluginOpener keeps the plugin open function in a var for easy testing
var pluginOpener = defaultPluginOpener

func defaultPluginOpener(name string) (Plugin, error) {
	return plugin.Open(name)
}

type loaderError struct {
	errors []error
}

// Error implements the error interface
func (l loaderError) Error() string {
	msgs := make([]string, len(l.errors))
	for i, err := range l.errors {
		msgs[i] = err.Error()
	}
	return fmt.Sprintf("plugin loader found %d error(s): \n%s", len(msgs), strings.Join(msgs, "\n"))
}

func (l loaderError) Len() int {
	return len(l.errors)
}

func (l loaderError) Errs() []error {
	return l.errors
}
