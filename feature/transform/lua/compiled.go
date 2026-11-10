package lua

import (
	"strings"

	"github.com/9edang/inkgate/internal/binder"
)

// Compiled is the code of a Lua config compiled once: the joined sources, the pre and the post script, with the
// source maps that turn their errors into file and line. It is only read after Compile, so the requests of an
// endpoint share one.
type Compiled struct {
	sources, pre, post      *binder.Chunk
	srcMap, preMap, postMap SourceMap
}

// Compile compiles the code of cfg. It fails when a source is missing or does not compile; the caller then keeps
// running the code per request, which reports the same error on every request.
func Compile(cfg Config) (*Compiled, error) {
	c := &Compiled{srcMap: NewSourceMap()}

	srcs := make([]string, 0, len(cfg.Sources))
	for _, name := range cfg.Sources {
		src, ok := cfg.Get(name)
		if !ok {
			return nil, ErrUnknownSource(name)
		}
		srcs = append(srcs, src)
		c.srcMap.Append(name, src)
	}

	var err error
	if c.sources, err = compileIf(strings.Join(srcs, "\n")); err != nil {
		return nil, err
	}
	if c.pre, err = compileIf(cfg.PreCode); err != nil {
		return nil, err
	}
	if c.post, err = compileIf(cfg.PostCode); err != nil {
		return nil, err
	}
	c.preMap = append(SourceMap{}, c.srcMap...)
	c.preMap.Append("pre-script", cfg.PreCode)
	c.postMap = append(SourceMap{}, c.srcMap...)
	c.postMap.Append("post-script", cfg.PostCode)

	return c, nil
}

func compileIf(src string) (*binder.Chunk, error) {
	if strings.TrimSpace(src) == "" {
		return nil, nil //nolint:nilnil // no code to run
	}

	return binder.Compile(src)
}

// RunSources runs the sources on b, as BinderWrapper.WithConfig does.
func (c *Compiled) RunSources(b *binder.Binder) error { return run(b, c.sources, &c.srcMap) }

// RunPre runs the pre script on b, as BinderWrapper.WithCode("pre-script", ...) does.
func (c *Compiled) RunPre(b *binder.Binder) error { return run(b, c.pre, &c.preMap) }

// RunPost runs the post script on b, as BinderWrapper.WithCode("post-script", ...) does.
func (c *Compiled) RunPost(b *binder.Binder) error { return run(b, c.post, &c.postMap) }

func run(b *binder.Binder, chunk *binder.Chunk, m *SourceMap) error {
	if chunk == nil {
		return nil
	}

	return ToError(b.Run(chunk), m)
}
