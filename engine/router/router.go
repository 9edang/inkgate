// SPDX-License-Identifier: Apache-2.0

/*
Package router defines some interfaces and common helpers for router adapters
*/
package router

import (
	"github.com/9edang/inkgate/context"
	"net/http"

	"github.com/9edang/inkgate/engine/config"
)

// Namespace is the service-level extra_config key for router options
// (error bodies, trusted proxies, health path, ...). Alias: "router".
const Namespace = "inkgate/engine/router"

// RunServerFunc starts the HTTP server with the router as handler.
// Every router variant and every server wrapper (CORS, OTel, plugins) uses this type.
type RunServerFunc func(context.Context, config.ServiceConfig, http.Handler) error

// Router sets up the public layer exposed to the users
type Router interface {
	Run(config.ServiceConfig)
}

// RouterFunc type is an adapter to allow the use of ordinary functions as routers.
// If f is a function with the appropriate signature, RouterFunc(f) is a Router that calls f.
type RouterFunc func(config.ServiceConfig)

// Run implements the Router interface
func (f RouterFunc) Run(cfg config.ServiceConfig) { f(cfg) }

// Factory creates new routers
type Factory interface {
	New() Router
	NewWithContext(context.Context) Router
}
