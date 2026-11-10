package proxy

import (
	"errors"
	"sync"

	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/engine/proxy"
	lua "github.com/9edang/inkgate/feature/transform/lua"
	"github.com/9edang/inkgate/feature/transform/lua/decorator"
	"github.com/9edang/inkgate/internal/binder"
)

// errNoResponse is raised by response.load() before the backend answered: a pre-script has no response.
var errNoResponse = errors.New("the response is not available before the backend call")

// vmPool runs the scripts of one endpoint or backend on reused Lua states. Building a state (decorators, tables) is
// most of the cost of a Lua request, so a state is built once and reused; the code is compiled once and shared.
//
// Isolation per request, the same as a fresh state:
//   - the sources chunk runs again on every request, so its locals and the functions closing over them are new;
//   - the globals are put back to their state after the decorators were registered;
//   - request and response are read from the state's current request, which is cleared after use.
//
// What survives on a state: tables a script reached through a global it did not create and modified in place, and
// modules loaded with require (their cache, as in any Lua program).
type vmPool struct {
	cfg  lua.Config
	code *lua.Compiled
	pool sync.Pool
}

// newVMPool compiles the code of cfg; see lua.Compile for when it fails.
func newVMPool(cfg lua.Config) (*vmPool, error) {
	code, err := lua.Compile(cfg)
	if err != nil {
		return nil, err
	}

	return &vmPool{cfg: cfg, code: code}, nil
}

// vm is one reused Lua state and the request it serves.
type vm struct {
	b       *binder.Binder
	globals binder.Globals
	ctx     context.Context
	req     *ProxyRequest
	resp    *ProxyResponse
}

func (p *vmPool) newVM() *vm {
	v := &vm{b: binder.New(binder.Options{SkipOpenLibs: !p.cfg.AllowOpenLibs, IncludeGoStackTrace: true})}

	decorator.RegisterErrors(v.b)
	decorator.RegisterNil(v.b)
	decorator.RegisterLuaTable(v.b)
	decorator.RegisterLuaList(v.b)
	decorator.RegisterHTTPRequestFunc(func() context.Context { return v.ctx }, v.b)
	for _, f := range localRegisterer.decorators {
		f(v.b)
	}
	registerRequestTableFunc(func() *ProxyRequest { return v.req }, v.b)
	registerResponseTableFunc(func() *ProxyResponse { return v.resp }, v.b)

	v.b.Init()
	v.globals = v.b.SnapshotGlobals()

	return v
}

func (p *vmPool) get() *vm {
	if v, ok := p.pool.Get().(*vm); ok {
		return v
	}

	return p.newVM()
}

// proxy serves requests the way New does on a fresh state, step for step.
func (p *vmPool) proxy(next proxy.Proxy) proxy.Proxy {
	return func(ctx context.Context, req *proxy.Request) (*proxy.Response, error) {
		v := p.get()
		v.ctx, v.req, v.resp = ctx, &ProxyRequest{req}, nil

		resp, err := p.serve(v, ctx, req, next)

		v.ctx, v.req, v.resp = nil, nil, nil // do not keep the request alive in an idle state
		// also after an error: a protected call leaves the state usable, the globals are reset here and the
		// sources run again on the next request, so a failed script leaves nothing behind that a successful one
		// would not. custom_error is ordinary control flow (a 401 from an auth script), so it must stay cheap.
		v.b.ResetGlobals(v.globals)
		p.pool.Put(v)

		return resp, err
	}
}

func (p *vmPool) serve(v *vm, ctx context.Context, req *proxy.Request, next proxy.Proxy) (*proxy.Response, error) {
	if err := p.code.RunSources(v.b); err != nil {
		return nil, err
	}
	if err := p.code.RunPre(v.b); err != nil {
		return nil, err
	}

	var resp *proxy.Response
	if !p.cfg.SkipNext {
		var err error
		resp, err = next(ctx, req)
		if err != nil {
			return resp, lua.ToError(err, nil)
		}
	} else {
		resp = &proxy.Response{}
	}

	v.resp = newProxyResponse(resp)
	if err := p.code.RunPost(v.b); err != nil {
		return nil, err
	}

	return resp, nil
}
