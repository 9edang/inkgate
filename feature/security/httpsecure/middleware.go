package httpsecure

import (
	"net/http"

	"github.com/unrolled/secure"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/router"
)

// NewSecureMw creates a secured middleware for the router engine.
func NewSecureMw(cfg config.ExtraConfig) router.HandlerMiddleware {
	opt, ok := ConfigGetter(cfg).(secure.Options)
	if !ok {
		return identityMiddleware{}
	}

	return secure.New(opt)
}

type identityMiddleware struct{}

func (i identityMiddleware) Handler(h http.Handler) http.Handler {
	return h
}
