package cors

import (
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/router"
)

// New returns a router.HandlerMiddleware with the CORS configuration defined in the ExtraConfig.
func New(e config.ExtraConfig) router.HandlerMiddleware {
	return NewWithLogger(e, nil)
}

// NewWithLogger is New with debug logging of CORS decisions.
func NewWithLogger(e config.ExtraConfig, l logging.Logger) router.HandlerMiddleware {
	if m := NewMiddleware(e, l); m != nil {
		return m
	}

	return nil
}
