package router

import (
	"net/http"
	"sync"

	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/router"
	lua "github.com/9edang/inkgate/feature/transform/lua"
	"github.com/9edang/inkgate/feature/transform/lua/decorator"
	"github.com/9edang/inkgate/internal/binder"
)

// processor runs the scripts of one config for a request.
type processor func(r *http.Request) error

// newProcessor returns the scripts of cfg running on reused, isolated Lua states (the same rules as the proxy
// module: the sources run again on every request, globals are put back, ctx is the request being served). With
// "live": true, or code that cannot be compiled (reported once here), every request builds a fresh state as before.
func newProcessor(l logging.Logger, logPrefix string, cfg lua.Config, pe router.ParamExtractor) processor {
	fresh := func(r *http.Request) error { return process(r, pe, &cfg) }
	if cfg.Live() {
		return fresh
	}
	code, err := lua.Compile(cfg)
	if err != nil {
		l.Error(logPrefix, "The Lua code cannot be loaded, every request will fail:", err.Error())
		return fresh
	}

	p := &vmPool{cfg: cfg, code: code, pe: pe}

	return p.process
}

type vmPool struct {
	cfg  lua.Config
	code *lua.Compiled
	pe   router.ParamExtractor
	pool sync.Pool
}

// vm is one reused Lua state and the request it serves.
type vm struct {
	b       *binder.Binder
	globals binder.Globals
	ctx     context.Context
	req     *requestContext
}

func (p *vmPool) newVM() *vm {
	v := &vm{b: binder.New(binder.Options{SkipOpenLibs: !p.cfg.AllowOpenLibs, IncludeGoStackTrace: true})}

	decorator.RegisterErrors(v.b)
	decorator.RegisterNil(v.b)
	decorator.RegisterLuaTable(v.b)
	decorator.RegisterLuaList(v.b)
	decorator.RegisterHTTPRequestFunc(func() context.Context { return v.ctx }, v.b)
	registerRequestTableFunc(func() *requestContext { return v.req }, v.b)

	v.b.Init()
	v.globals = v.b.SnapshotGlobals()

	return v
}

func (p *vmPool) process(r *http.Request) error {
	v, ok := p.pool.Get().(*vm)
	if !ok {
		v = p.newVM()
	}
	v.ctx, v.req = r.Context(), &requestContext{Request: r, pe: p.pe}

	err := p.code.RunSources(v.b)
	if err == nil {
		err = p.code.RunPre(v.b)
	}

	v.ctx, v.req = nil, nil // do not keep the request alive in an idle state
	v.b.ResetGlobals(v.globals)
	p.pool.Put(v)

	return err
}
