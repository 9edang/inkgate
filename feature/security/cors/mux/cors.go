package mux

import (
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	mux "github.com/9edang/inkgate/engine/router/mux"
	inkgatecors "github.com/9edang/inkgate/feature/security/cors"
)

// New returns a mux.HandlerMiddleware with the CORS configuration defined in the ExtraConfig.
func New(e config.ExtraConfig) mux.HandlerMiddleware {
	return NewWithLogger(e, nil)
}

// NewWithLogger is New with debug logging of CORS decisions.
func NewWithLogger(e config.ExtraConfig, l logging.Logger) mux.HandlerMiddleware {
	if m := inkgatecors.NewMiddleware(e, l); m != nil {
		return m
	}
	return nil
}
