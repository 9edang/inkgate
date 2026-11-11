package router

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/textproto"
	"net/url"

	"github.com/9edang/inkgate/internal/constant"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	"github.com/9edang/inkgate/engine/router"
	lua "github.com/9edang/inkgate/feature/transform/lua"
	"github.com/9edang/inkgate/feature/transform/lua/decorator"

	glua "github.com/yuin/gopher-lua"

	"github.com/9edang/inkgate/internal/binder"
)

func RegisterMiddleware(l logging.Logger, e config.ExtraConfig, pe router.ParamExtractor, mws []router.HandlerMiddleware) []router.HandlerMiddleware {
	logPrefix := "[Service: Router][Lua]"
	cfg, err := lua.Parse(l, e, Namespace)
	if err != nil {
		if !errors.Is(err, lua.ErrNoExtraConfig) {
			l.Debug(logPrefix, err.Error())
		}

		return mws
	}

	l.Debug(logPrefix, "Middleware is now ready")

	return append(mws, &middleware{process: newProcessor(l, logPrefix, cfg, pe)})
}

type middleware struct {
	process processor
}

func (hm *middleware) Handler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := hm.process(r); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		h.ServeHTTP(w, r)
	})
}

func HandlerFactory(l logging.Logger, next router.HandlerFactory, pe router.ParamExtractor) router.HandlerFactory {
	return func(remote *config.EndpointConfig, p proxy.Proxy) http.HandlerFunc {
		logPrefix := "[ENDPOINT: " + remote.Endpoint + "][Lua]"
		handlerFunc := next(remote, p)

		cfg, err := lua.Parse(l, remote.ExtraConfig, Namespace)
		if err != nil {
			if !errors.Is(err, lua.ErrNoExtraConfig) {
				l.Debug(logPrefix, err.Error())
			}

			return handlerFunc
		}

		l.Debug(logPrefix, "Middleware is now ready")
		run := newProcessor(l, logPrefix, cfg, pe)

		return func(w http.ResponseWriter, r *http.Request) {
			if err := run(r); err != nil {
				var errhttp errHTTP
				if errors.As(err, &errhttp) {
					var e errHTTPWithContentType
					if errors.As(err, &e) {
						w.Header().Add(constant.HeaderContentType, e.Encoding())
					}
					w.WriteHeader(errhttp.StatusCode())
					// #nosec G104 -- response write is best effort, the client may be gone
					w.Write([]byte(err.Error()))

					return
				}

				http.Error(w, err.Error(), http.StatusInternalServerError)

				return
			}

			handlerFunc(w, r)
		}
	}
}

type errHTTP interface {
	error
	StatusCode() int
}

type errHTTPWithContentType interface {
	errHTTP
	Encoding() string
}

func process(r *http.Request, pe router.ParamExtractor, cfg *lua.Config) error {
	b := lua.NewBinderWrapper(binder.Options{
		SkipOpenLibs:        !cfg.AllowOpenLibs,
		IncludeGoStackTrace: true,
	})

	decorator.RegisterErrors(b.GetBinder())
	decorator.RegisterNil(b.GetBinder())
	decorator.RegisterLuaTable(b.GetBinder())
	decorator.RegisterLuaList(b.GetBinder())
	decorator.RegisterHTTPRequest(r.Context(), b.GetBinder())
	registerRequestTable(r, pe, b.GetBinder())

	if err := b.WithConfig(cfg); err != nil {
		return err
	}

	return b.WithCode("pre-script", cfg.PreCode)
}

func registerRequestTable(r *http.Request, pe router.ParamExtractor, b *binder.Binder) {
	mctx := &requestContext{
		Request: r,
		pe:      pe,
	}
	registerRequestTableFunc(func() *requestContext { return mctx }, b)
}

// registerRequestTableFunc registers the ctx table; ctx.load() returns current(), so a reused state serves the
// request it is running for.
func registerRequestTableFunc(current func() *requestContext, b *binder.Binder) {
	t := b.Table("ctx")

	t.Static("load", func(c *binder.Context) error {
		mctx := current()
		if mctx == nil {
			return errContextExpected
		}
		c.Push().Data(mctx, "ctx")
		return nil
	})

	var mctx *requestContext // the methods read the request from their first argument, not from the receiver
	t.Dynamic("method", mctx.method)
	t.Dynamic("url", mctx.url)
	t.Dynamic("query", mctx.query)
	t.Dynamic("params", mctx.params)
	t.Dynamic("headers", mctx.headers)
	t.Dynamic("headerList", mctx.headerList)
	t.Dynamic("body", mctx.body)
}

type requestContext struct {
	*http.Request
	pe router.ParamExtractor
}

func (*requestContext) method(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*requestContext)
	if !ok {
		return errContextExpected
	}

	if c.Top() == 1 {
		c.Push().String(req.Method)
	} else {
		req.Method = c.Arg(2).String()
	}

	return nil
}

func (*requestContext) url(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*requestContext)
	if !ok {
		return errContextExpected
	}

	if c.Top() == 1 {
		c.Push().String(req.URL.String())
	} else {
		req.URL, _ = url.Parse(c.Arg(2).String())
	}

	return nil
}

func (*requestContext) query(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*requestContext)
	if !ok {
		return errContextExpected
	}

	switch c.Top() {
	case 1:
		return errNeedsArguments
	case 2:
		c.Push().String(req.URL.Query().Get(c.Arg(2).String()))
	case 3:
		q := req.URL.Query()
		q.Set(c.Arg(2).String(), c.Arg(3).String())
		req.URL.RawQuery = q.Encode()
	}

	return nil
}

func (*requestContext) params(_ *binder.Context) error {
	return nil
}

func (*requestContext) headers(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*requestContext)
	if !ok {
		return errContextExpected
	}
	switch c.Top() {
	case 1:
		c.Push().Data(lua.NewTableFromStringSliceMap(req.Request.Header), "luaTable")
	case 2:
		c.Push().String(req.Header.Get(c.Arg(2).String()))
	case 3:
		_, isNil := c.Arg(3).Any().(*glua.LNilType)
		if isNil {
			req.Header.Del(c.Arg(2).String())
			return nil
		}
		req.Header.Set(c.Arg(2).String(), c.Arg(3).String())
	}

	return nil
}

func (*requestContext) headerList(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*requestContext)
	if !ok {
		return errContextExpected
	}
	switch c.Top() {
	case 1:
		return errNeedsArguments
	case 2:
		key := textproto.CanonicalMIMEHeaderKey(c.Arg(2).String())

		headers := req.Header.Values(key)
		d := make([]any, len(headers))
		for i := range headers {
			d[i] = headers[i]
		}
		c.Push().Data(&lua.List{Data: d}, "luaList")
	case 3:
		key := textproto.CanonicalMIMEHeaderKey(c.Arg(2).String())

		v, isUserData := c.Arg(3).Any().(*glua.LUserData)
		if !isUserData {
			return errInvalidLuaList
		}

		list, isList := v.Value.(*lua.List)
		if !isList {
			return errInvalidLuaList
		}

		d := make([]string, len(list.Data))
		for i := range list.Data {
			d[i] = fmt.Sprintf("%s", list.Data[i])
		}
		req.Header[key] = d
	}

	return nil
}

func (*requestContext) body(c *binder.Context) error {
	req, ok := c.Arg(1).Data().(*requestContext)
	if !ok {
		return errContextExpected
	}

	if c.Top() == 2 {
		req.Body = io.NopCloser(bytes.NewBufferString(c.Arg(2).String()))
		return nil
	}

	var b []byte
	if req.Body != nil {
		b, _ = io.ReadAll(req.Body)
		// #nosec G104 -- closing is best effort, the error is not actionable
		req.Body.Close()
	}
	req.Body = io.NopCloser(bytes.NewBuffer(b))
	c.Push().String(string(b))

	return nil
}

var (
	errNeedsArguments  = errors.New("need arguments")
	errContextExpected = errors.New("requestContext expected")
	errInvalidLuaList  = errors.New("invalid header value, must be a luaList")
)
