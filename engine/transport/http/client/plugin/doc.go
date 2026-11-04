// SPDX-License-Identifier: Apache-2.0

// Package plugin provides plugin register interfaces for building http client plugins.
//
// Usage example:
//
//	package main
//
//	import (
//		"context"
//		"errors"
//		"fmt"
//		"html"
//		"net/http"
//	)
//
//	// ClientRegisterer is the symbol the plugin loader will try to load. It must implement the RegisterClient interface
//	var ClientRegisterer = registerer("inkgate-example")
//
//	type registerer string
//
//	func (r registerer) RegisterClients(f func(
//		name string,
//		handler func(context.Context, map[string]any) (http.Handler, error),
//	)) {
//		f(string(r), r.registerClients)
//	}
//
//	func (r registerer) registerClients(ctx context.Context, extra map[string]any) (http.Handler, error) {
//		// check the passed configuration and initialize the plugin
//		name, ok := extra["name"].(string)
//		if !ok {
//			return nil, errors.New("wrong config")
//		}
//		if name != string(r) {
//			return nil, fmt.Errorf("unknown register %s", name)
//		}
//		// return the actual handler wrapping or your custom logic so it can be used as a replacement for the default http client
//		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
//			fmt.Fprintf(w, "Hello, %q", html.EscapeString(req.URL.Path))
//		}), nil
//	}
//
//	func init() {
//		fmt.Println("inkgate-example client plugin loaded!!!")
//	}
//
//	func main() {}
//
// To be usable in a list of plugins ("name": ["one", "two"]) the handler also
// receives the next stage, to call it (the last one is the regular backend call)
// or to answer by itself. The method name is the same, only the handler type
// differs; plugins written as above keep working and end the chain:
//
//	func (r registerer) RegisterClients(f func(
//		name string,
//		handler func(context.Context, map[string]any, http.Handler) (http.Handler, error),
//	)) {
//		f(string(r), func(ctx context.Context, extra map[string]any, next http.Handler) (http.Handler, error) {
//			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
//				req.Header.Set("X-Example", "1")
//				next.ServeHTTP(w, req)
//			}), nil
//		})
//	}
package plugin
