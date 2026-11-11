package proxy

import (
	"errors"
	"github.com/9edang/inkgate/context"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	lua "github.com/9edang/inkgate/feature/transform/lua"
	"github.com/9edang/inkgate/feature/transform/lua/decorator"
	"github.com/9edang/inkgate/internal/binder"
)

const (
	ProxyNamespace   = "inkgate/feature/transform/lua/proxy"
	BackendNamespace = "inkgate/feature/transform/lua/proxy/backend"
)

func ProxyFactory(l logging.Logger, pf proxy.Factory) proxy.Factory {
	return proxy.FactoryFunc(func(remote *config.EndpointConfig) (proxy.Proxy, error) {
		logPrefix := "[ENDPOINT: " + remote.Endpoint + "][Lua]"
		next, err := pf.New(remote)
		if err != nil {
			return next, err
		}

		cfg, err := lua.Parse(l, remote.ExtraConfig, ProxyNamespace)
		if err != nil {
			if err != lua.ErrNoExtraConfig {
				l.Debug(logPrefix, err)
			}
			return next, nil
		}

		l.Debug(logPrefix, "Middleware is now ready")

		return newLogged(l, logPrefix, cfg, next), nil
	})
}

func BackendFactory(l logging.Logger, bf proxy.BackendFactory) proxy.BackendFactory {
	return func(remote *config.Backend) proxy.Proxy {
		logPrefix := "[BACKEND: " + remote.URLPattern + "][Lua]"
		next := bf(remote)

		cfg, err := lua.Parse(l, remote.ExtraConfig, BackendNamespace)
		if err != nil {
			if err != lua.ErrNoExtraConfig {
				l.Debug(logPrefix, err)
			}
			return next
		}

		return newLogged(l, logPrefix, cfg, next)
	}
}

type registerer struct {
	decorators []decorator.Decorator
}

var localRegisterer = registerer{decorators: []decorator.Decorator{}}

func RegisterDecorator(f decorator.Decorator) {
	localRegisterer.decorators = append(localRegisterer.decorators, f)
}

// New wraps next with the Lua scripts of cfg. The scripts run on reused, isolated Lua states (see vmPool); with
// "live": true, or when the code cannot be compiled ahead, every request builds a fresh state as before.
func New(cfg lua.Config, next proxy.Proxy) proxy.Proxy {
	if !cfg.Live() {
		if p, err := newVMPool(cfg); err == nil {
			return p.proxy(next)
		}
	}

	return newFresh(cfg, next)
}

// newLogged is New that reports, once at startup, code that cannot be compiled: otherwise the only sign is an
// error on every request.
func newLogged(l logging.Logger, logPrefix string, cfg lua.Config, next proxy.Proxy) proxy.Proxy {
	if cfg.Live() {
		return newFresh(cfg, next)
	}
	p, err := newVMPool(cfg)
	if err != nil {
		l.Error(logPrefix, "The Lua code cannot be loaded, every request will fail:", err.Error())
		return newFresh(cfg, next)
	}

	return p.proxy(next)
}

// newFresh builds a new Lua state for every request.
func newFresh(cfg lua.Config, next proxy.Proxy) proxy.Proxy {
	return func(ctx context.Context, req *proxy.Request) (resp *proxy.Response, err error) {
		b := lua.NewBinderWrapper(binder.Options{
			SkipOpenLibs:        !cfg.AllowOpenLibs,
			IncludeGoStackTrace: true,
		})
		defer b.GetBinder().Close()

		decorator.RegisterErrors(b.GetBinder())
		decorator.RegisterNil(b.GetBinder())
		decorator.RegisterLuaTable(b.GetBinder())
		decorator.RegisterLuaList(b.GetBinder())
		decorator.RegisterHTTPRequest(ctx, b.GetBinder())
		for _, f := range localRegisterer.decorators {
			f(b.GetBinder())
		}

		registerRequestTable(req, b.GetBinder())

		if err := b.WithConfig(&cfg); err != nil {
			return nil, err
		}

		if err := b.WithCode("pre-script", cfg.PreCode); err != nil {
			return nil, err
		}

		if !cfg.SkipNext {
			resp, err = next(ctx, req)
			if err != nil {
				return resp, lua.ToError(err, nil)
			}
		} else {
			resp = &proxy.Response{}
		}

		registerResponseTable(resp, b.GetBinder())

		if err := b.WithCode("post-script", cfg.PostCode); err != nil {
			return nil, err
		}

		return resp, nil
	}
}

var (
	errNeedsArguments = errors.New("need arguments")
	errInvalidLuaList = errors.New("invalid header value, must be a luaList")
)
