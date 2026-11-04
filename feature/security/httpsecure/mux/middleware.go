package mux

import (
	"net/http"

	"github.com/9edang/inkgate/engine/config"
	mux "github.com/9edang/inkgate/engine/router/mux"
	"github.com/unrolled/secure"

	httpsecure "github.com/9edang/inkgate/feature/security/httpsecure"
)

// NewSecureMw creates a secured middleware for the mux engine
func NewSecureMw(cfg config.ExtraConfig) mux.HandlerMiddleware {
	opt, ok := httpsecure.ConfigGetter(cfg).(secure.Options)
	if !ok {
		return identityMiddleware{}
	}

	return secure.New(opt)
}

type identityMiddleware struct{}

func (i identityMiddleware) Handler(h http.Handler) http.Handler {
	return h
}
