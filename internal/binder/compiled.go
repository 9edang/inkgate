package binder

import (
	"strings"

	lua "github.com/yuin/gopher-lua"
	"github.com/yuin/gopher-lua/parse"
)

// Chunk is Lua code compiled once and run many times, on any Binder. A FunctionProto is immutable, so one Chunk is
// safe to share between states.
type Chunk struct {
	src   string
	proto *lua.FunctionProto
}

// Compile compiles src the way DoString does (same chunk name), so errors and their positions read the same.
func Compile(src string) (*Chunk, error) {
	const name = "<string>" // the chunk name gopher-lua's DoString uses
	ast, err := parse.Parse(strings.NewReader(src), name)
	if err != nil {
		return nil, err
	}
	proto, err := lua.Compile(ast, name)
	if err != nil {
		return nil, err
	}

	return &Chunk{src: src, proto: proto}, nil
}

// Init registers the loaders (functions, modules, tables) in the state. DoString does it on every call; a reused
// Binder does it once.
func (b *Binder) Init() {
	b.load()
}

// Run runs a compiled chunk. It behaves like DoString(c.src) without parsing and compiling, and leaves the stack
// empty, so a reused state does not accumulate returned values.
func (b *Binder) Run(c *Chunk) error {
	defer b.state.SetTop(0)
	b.state.Push(b.state.NewFunctionFromProto(c.proto))

	return b.do(b.state.PCall(0, lua.MultRet, nil), func(problem int) *source {
		return newSource(c.src, problem)
	})
}

// Globals is a snapshot of the global table.
type Globals map[lua.LValue]lua.LValue

// SnapshotGlobals records the global table as it is now.
func (b *Binder) SnapshotGlobals() Globals {
	g := Globals{}
	b.state.G.Global.ForEach(func(k, v lua.LValue) { g[k] = v })

	return g
}

// ResetGlobals puts the global table back to the snapshot: globals created since are removed, changed ones are
// restored. Values are compared by identity, so a table that was modified in place is not restored; state kept
// that way survives (re-run the chunks that create it to get fresh copies).
func (b *Binder) ResetGlobals(g Globals) {
	global := b.state.G.Global
	changed := map[lua.LValue]lua.LValue{}
	global.ForEach(func(k, v lua.LValue) { // collect first: the table is not modified while iterated
		if snap, ok := g[k]; !ok {
			changed[k] = lua.LNil
		} else if snap != v {
			changed[k] = snap
		}
	})
	for k, v := range g {
		if global.RawGet(k) == lua.LNil {
			changed[k] = v // deleted by the script
		}
	}
	for k, v := range changed {
		global.RawSet(k, v)
	}
}
