package decorator

import (
	"bytes"
	"errors"
	"github.com/9edang/inkgate/context"
	"net/http"
	"sync"

	lua "github.com/9edang/inkgate/feature/transform/lua"
	"github.com/9edang/inkgate/internal/binder"

	"github.com/9edang/inkgate/engine/transport/http/server"
)

func RegisterHTTPRequest(ctx context.Context, b *binder.Binder) {
	RegisterHTTPRequestFunc(func() context.Context { return ctx }, b)
}

// RegisterHTTPRequestFunc is RegisterHTTPRequest for a binder reused across requests: ctx is called on every
// http_response.new, so each call uses the context of the request being served.
func RegisterHTTPRequestFunc(ctx func() context.Context, b *binder.Binder) {
	t := b.Table("http_response")

	t.Static("new", newHttpResponse(ctx))

	t.Dynamic("statusCode", httpStatus)
	t.Dynamic("headers", httpHeaders)
	t.Dynamic("headerList", httpHeaderList)
	t.Dynamic("body", httpBody)
	t.Dynamic("close", httpClose)
}

func newHttpResponse(ctx func() context.Context) func(*binder.Context) error {
	return func(c *binder.Context) error {
		if c.Top() == 0 || c.Top() == 2 {
			return errors.New("need 1, 3 or 4 arguments")
		}

		URL := c.Arg(1).String()
		var req *http.Request

		if c.Top() == 1 {

			req, _ = http.NewRequest("GET", URL, http.NoBody)

		} else {

			method := c.Arg(2).String()
			body := c.Arg(3).String()

			var err error
			req, err = http.NewRequest(method, URL, bytes.NewBufferString(body))
			if err != nil {
				return err
			}

			if c.Top() == 4 {
				headers, ok := c.Arg(4).Any().(*lua.NativeTable)

				if ok {
					headers.ForEach(func(key, value lua.NativeValue) {
						switch l := value.(type) {
						case lua.NativeString:
							req.Header.Add(key.String(), l.String())
						case *lua.NativeTable:
							l.ForEach(func(_, v lua.NativeValue) {
								req.Header.Add(key.String(), v.String())
							})
						}
					})
				}
			}
		}

		// the body is handed to the script, which closes it with resp:close(); if it does not, the request context
		// ends the connection when the gateway request is done
		//nolint:bodyclose // ownership passes to the Lua script
		resp, err := executeHttpRequest(req.WithContext(ctx()))
		if err != nil {
			return err
		}
		if resp == nil {
			return ErrResponseExpected
		}
		pushHTTPResponse(c, resp)
		return nil
	}
}

func executeHttpRequest(r *http.Request) (*http.Response, error) {
	r.Header.Add("User-Agent", server.UserAgentHeaderValue[0])
	return http.DefaultClient.Do(r)
}

func pushHTTPResponse(c *binder.Context, r *http.Response) {
	c.Push().Data(
		&lua.HttpResponse{
			Once: new(sync.Once),
			R:    r,
		},
		"http_response",
	)
}

func httpStatus(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*lua.HttpResponse)
	if !ok {
		return ErrResponseExpected
	}
	c.Push().Number(float64(resp.R.StatusCode))

	return nil
}

func httpHeaders(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*lua.HttpResponse)
	if !ok {
		return ErrResponseExpected
	}
	if c.Top() != 2 {
		return ErrNeedsArguments
	}
	c.Push().String(resp.Header(c.Arg(2).String()))

	return nil
}

func httpHeaderList(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*lua.HttpResponse)
	if !ok {
		return ErrResponseExpected
	}
	if c.Top() != 2 {
		return ErrNeedsArguments
	}

	headers := resp.Headers(c.Arg(2).String())
	d := make([]any, len(headers))
	for i := range headers {
		d[i] = headers[i]
	}
	c.Push().Data(&lua.List{Data: d}, "luaList")

	return nil
}

func httpBody(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*lua.HttpResponse)
	if !ok {
		return ErrResponseExpected
	}
	c.Push().String(resp.Body())

	return nil
}

func httpClose(c *binder.Context) error {
	resp, ok := c.Arg(1).Data().(*lua.HttpResponse)
	if !ok {
		return ErrResponseExpected
	}
	if resp == nil {
		return nil
	}
	resp.Close()
	resp.R = nil
	resp = nil
	return nil
}
