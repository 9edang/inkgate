package proxy

import (
	"fmt"
	"strconv"
	"sync"
	"testing"

	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/engine/proxy"
	lua "github.com/9edang/inkgate/feature/transform/lua"
)

// memSources is a SourceLoader for tests: sources by name, without files.
type memSources map[string]string

func (m memSources) Get(k string) (string, bool) { v, ok := m[k]; return v, ok }

func echoBackend(_ context.Context, req *proxy.Request) (*proxy.Response, error) {
	return &proxy.Response{Data: map[string]any{"id": req.Params["Id"]}, IsComplete: true}, nil
}

func pooled(t *testing.T, cfg lua.Config) proxy.Proxy {
	t.Helper()
	p, err := newVMPool(cfg)
	if err != nil {
		t.Fatal(err)
	}

	return p.proxy(echoBackend)
}

func call(p proxy.Proxy, id string) (*proxy.Response, error) {
	return p(context.Background(), &proxy.Request{Params: map[string]string{"Id": id}, Headers: map[string][]string{}})
}

// A global a script creates in one request is gone in the next.
func TestVMPoolGlobalsDoNotLeak(t *testing.T) {
	p := pooled(t, lua.Config{
		PreCode:  `if leaked ~= nil then custom_error("global leaked: " .. leaked, 500) end`,
		PostCode: `leaked = "from a previous request"`,
	})
	for i := range 5 {
		if _, err := call(p, strconv.Itoa(i)); err != nil {
			t.Fatalf("request %d: %v", i, err)
		}
	}
}

// State a source keeps in a local (an upvalue of its functions) starts fresh on every request, as on a new state.
func TestVMPoolSourceLocalsAreFresh(t *testing.T) {
	p := pooled(t, lua.Config{
		Sources:      []string{"counter.lua"},
		SourceLoader: memSources{"counter.lua": "local seen = 0\nfunction bump() seen = seen + 1; return seen end"},
		PostCode:     `response.load():data("calls", bump())`,
	})
	for i := range 5 {
		resp, err := call(p, strconv.Itoa(i))
		if err != nil {
			t.Fatal(err)
		}
		if got := resp.Data["calls"]; got != 1.0 {
			t.Fatalf("request %d: calls = %v, want 1 (state carried over)", i, got)
		}
	}
}

// Before the backend answered there is no response, also on a state that served a request before.
func TestVMPoolNoResponseBeforeBackend(t *testing.T) {
	p := pooled(t, lua.Config{PreCode: `response.load()`})
	for i := range 2 {
		if _, err := call(p, "1"); err == nil {
			t.Fatalf("request %d: response.load() in a pre-script must fail", i)
		}
	}
}

// Concurrent requests on the pool each see their own request and response.
func TestVMPoolConcurrentRequestsAreIsolated(t *testing.T) {
	p := pooled(t, lua.Config{
		PreCode: `local r = request.load(); r:headers("X-Seen", r:params("Id"))`,
		PostCode: `local d = response.load():data()
d:set("from_lua", request.load():params("Id"))`,
	})
	var wg sync.WaitGroup
	errs := make(chan error, 200)
	for i := range 200 {
		wg.Go(func() {
			id := strconv.Itoa(i)
			resp, err := call(p, id)
			if err != nil {
				errs <- err
				return
			}
			if resp.Data["id"] != id || resp.Data["from_lua"] != id {
				errs <- fmt.Errorf("request %s got %v", id, resp.Data)
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

// A request that fails does not poison the next one.
func TestVMPoolRecoversAfterError(t *testing.T) {
	p := pooled(t, lua.Config{PreCode: `local r = request.load()
if r:params("Id") == "bad" then error("boom") end`})
	if _, err := call(p, "bad"); err == nil {
		t.Fatal("expected an error")
	}
	if _, err := call(p, "good"); err != nil {
		t.Fatalf("the request after a failure: %v", err)
	}
}

// Errors read the same as on a fresh state: message and, for custom_error, status code.
func TestVMPoolErrorsMatchFreshState(t *testing.T) {
	for _, code := range []string{
		`custom_error("nope", 418)`,
		`error("plain lua error")`,
		`local x = nil; x.field = 1`,
	} {
		cfg := lua.Config{PreCode: code, SourceLoader: memSources{}}
		_, want := call(newFresh(cfg, echoBackend), "1")
		_, got := call(pooled(t, cfg), "1")
		if want == nil || got == nil {
			t.Fatalf("%s: both paths must fail (fresh %v, pooled %v)", code, want, got)
		}
		if got.Error() != want.Error() {
			t.Errorf("%s: message\n pooled %q\n fresh  %q", code, got, want)
		}
		ws, wok := want.(interface{ StatusCode() int })
		gs, gok := got.(interface{ StatusCode() int })
		if wok != gok || (wok && ws.StatusCode() != gs.StatusCode()) {
			t.Errorf("%s: status differs (fresh %v, pooled %v)", code, want, got)
		}
	}
}
