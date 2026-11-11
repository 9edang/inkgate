// bloop rewrites `for i := 0; i < b.N; i++ {` and `for range b.N {` into `for b.Loop() {` where that is safe, and
// reports every loop it leaves alone with the reason.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
)

type edit struct {
	from, to int
	text     string
}

type loop struct {
	stmt    ast.Stmt
	forPos  token.Pos
	lbrace  token.Pos
	body    *ast.BlockStmt
	varName string
	reset   *ast.ExprStmt // a b.ResetTimer() right before the loop in the same block
}

func benchParam(ft *ast.FuncType) string {
	for _, f := range ft.Params.List {
		star, ok := f.Type.(*ast.StarExpr)
		if !ok || len(f.Names) != 1 {
			continue
		}
		if sel, ok := star.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "B" {
			if id, ok := sel.X.(*ast.Ident); ok && id.Name == "testing" {
				return f.Names[0].Name
			}
		}
	}

	return ""
}

func isBN(e ast.Node, name string) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "N" {
		return false
	}
	id, ok := sel.X.(*ast.Ident)

	return ok && id.Name == name
}

func isResetTimer(s ast.Stmt, name string) *ast.ExprStmt {
	es, ok := s.(*ast.ExprStmt)
	if !ok {
		return nil
	}
	call, ok := es.X.(*ast.CallExpr)
	if !ok {
		return nil
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "ResetTimer" {
		return nil
	}
	if id, ok := sel.X.(*ast.Ident); ok && id.Name == name {
		return es
	}

	return nil
}

func main() {
	changed, skipped := 0, 0
	for _, path := range os.Args[1:] {
		src, err := os.ReadFile(path)
		if err != nil {
			panic(err)
		}
		fset := token.NewFileSet()
		f, err := parser.ParseFile(fset, path, src, parser.ParseComments)
		if err != nil {
			panic(err)
		}
		var edits []edit
		report := func(pos token.Pos, why string) {
			skipped++
			fmt.Printf("SKIP %s:%d %s\n", path, fset.Position(pos).Line, why)
		}
		off := func(p token.Pos) int { return fset.Position(p).Offset }

		var visit func(ft *ast.FuncType, body *ast.BlockStmt)
		visit = func(ft *ast.FuncType, body *ast.BlockStmt) {
			if body == nil {
				return
			}
			name := benchParam(ft)
			var loops []loop
			totalBN, inClosure := 0, false

			ast.Inspect(body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.FuncLit:
					if benchParam(x.Type) != "" {
						visit(x.Type, x.Body) // a sub-benchmark has its own b
					} else if name != "" {
						ast.Inspect(x.Body, func(m ast.Node) bool {
							if isBN(m, name) {
								inClosure = true
								totalBN++
							}
							return true
						})
						visit(x.Type, x.Body)
					} else {
						visit(x.Type, x.Body)
					}

					return false
				case *ast.BlockStmt:
					if name == "" {
						return true
					}
					for i, st := range x.List {
						var l *loop
						switch s := st.(type) {
						case *ast.ForStmt:
							if c, ok := s.Cond.(*ast.BinaryExpr); ok && c.Op == token.LSS && isBN(c.Y, name) {
								l = &loop{stmt: s, forPos: s.For, lbrace: s.Body.Lbrace, body: s.Body}
								if as, ok := s.Init.(*ast.AssignStmt); ok && len(as.Lhs) == 1 {
									if id, ok := as.Lhs[0].(*ast.Ident); ok {
										l.varName = id.Name
									}
								}
							}
						case *ast.RangeStmt:
							if isBN(s.X, name) {
								l = &loop{stmt: s, forPos: s.For, lbrace: s.Body.Lbrace, body: s.Body}
								if id, ok := s.Key.(*ast.Ident); ok && id.Name != "_" {
									l.varName = id.Name
								}
							}
						}
						if l != nil {
							if i > 0 {
								l.reset = isResetTimer(x.List[i-1], name)
							}
							loops = append(loops, *l)
						}
					}
				case *ast.SelectorExpr:
					if name != "" && isBN(x, name) {
						totalBN++
					}
				}

				return true
			})
			if len(loops) == 0 {
				return
			}
			switch {
			case len(loops) > 1:
				for _, l := range loops {
					report(l.forPos, "more than one b.N loop in this benchmark (b.Loop runs once)")
				}
				return
			case inClosure:
				report(loops[0].forPos, "b.N is also used inside a closure or goroutine")
				return
			case totalBN != 1:
				report(loops[0].forPos, "b.N is used elsewhere in the benchmark")
				return
			}
			l := loops[0]
			if l.varName != "" {
				used := false
				ast.Inspect(l.body, func(m ast.Node) bool {
					if id, ok := m.(*ast.Ident); ok && id.Name == l.varName {
						used = true
					}
					return true
				})
				if used {
					report(l.forPos, "the loop variable "+l.varName+" is used in the body")
					return
				}
			}
			from := off(l.forPos)
			if l.reset != nil {
				from = off(l.reset.Pos()) // b.Loop resets the timer on its first call
			}
			edits = append(edits, edit{from, off(l.lbrace), "for b.Loop() "})
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				visit(fd.Type, fd.Body)
			}
		}
		if len(edits) == 0 {
			continue
		}
		sort.Slice(edits, func(i, j int) bool { return edits[i].from > edits[j].from })
		out := src
		for _, e := range edits {
			out = append(append(append([]byte{}, out[:e.from]...), e.text...), out[e.to:]...)
		}
		if err := os.WriteFile(path, out, 0o644); err != nil {
			panic(err)
		}
		changed++
		fmt.Printf("OK   %s (%d)\n", path, len(edits))
	}
	fmt.Printf("files changed: %d, loops skipped: %d\n", changed, skipped)
}
