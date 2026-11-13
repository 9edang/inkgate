// SPDX-License-Identifier: Apache-2.0

package boot

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/9edang/inkgate/schema"
)

// notConfigKeys are Namespace constants that are not extra_config keys, each with the reason.
var notConfigKeys = map[string]string{
	"inkgate/engine/proxy/plugin/request":  "internal name of the request modifier register",
	"inkgate/engine/proxy/plugin/response": "internal name of the response modifier register",
	"dns":                                  "name of the dns service-discovery module, picked with \"sd\" in a backend",
	"backend/grpc":                         "only read by `inkgate audit`, to count a KrakenD gRPC backend as unsafe; inkgate has no gRPC backend",
	// the package exists but no flow builds it, so a config naming it would do nothing: refusing it is right
	"inkgate/feature/traffic/circuitbreaker/eapache": "not wired into the backend flow; use qos/circuit-breaker",
}

type extraRead struct {
	dir, path string
	key       ast.Expr
}

// The known namespaces must be the ones the code reads. This finds every `Namespace` constant of the module, and every
// key read straight off an ExtraConfig field, and fails on one that is not known, so a feature added later cannot be left out and then refused as a typo.
func TestKnownNamespaces_coverEveryNamespaceConstant(t *testing.T) {
	known := KnownNamespaces()
	found := map[string]string{}  // value -> where
	consts := map[string]string{} // "dir|NAME" -> value, for the package-level string constants
	var reads []extraRead
	fset := token.NewFileSet()
	err := filepath.WalkDir("../..", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return nil // a file that does not parse is not this test's business
		}
		dir := filepath.Dir(path)
		for _, decl := range f.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					lit, ok := vs.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						continue
					}
					v, _ := strconv.Unquote(lit.Value)
					consts[dir+"|"+name.Name] = v
					if strings.HasSuffix(strings.ToLower(name.Name), "namespace") {
						found[v] = path + ": " + name.Name
					}
				}
			}
		}
		// keys read straight off an ExtraConfig field, by literal or by a constant of the same package:
		// remote.ExtraConfig[clientHTTPOptions] is a namespace whatever the constant is called
		ast.Inspect(f, func(n ast.Node) bool {
			ix, ok := n.(*ast.IndexExpr)
			if !ok {
				return true
			}
			if sel, ok := ix.X.(*ast.SelectorExpr); ok && sel.Sel.Name == "ExtraConfig" {
				reads = append(reads, extraRead{dir: dir, path: path, key: ix.Index})
			}

			return true
		})

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range reads {
		switch k := r.key.(type) {
		case *ast.BasicLit:
			if v, err := strconv.Unquote(k.Value); err == nil {
				found[v] = r.path + ": ExtraConfig[" + k.Value + "]"
			}
		case *ast.Ident:
			if v, ok := consts[r.dir+"|"+k.Name]; ok {
				found[v] = r.path + ": ExtraConfig[" + k.Name + "]"
			}
		}
	}
	if len(found) < 30 {
		t.Fatalf("found only %d namespace constants: the walk is broken", len(found))
	}
	for v, where := range found {
		_, internal := notConfigKeys[v]
		if !slices.Contains(known, v) && !internal {
			t.Errorf("%s = %q is not in KnownNamespaces: add an alias in main.go (or list it in unaliased), or, if it is not an extra_config key, in notConfigKeys with the reason", where, v)
		}
	}
	for v := range notConfigKeys {
		if _, ok := found[v]; !ok {
			t.Errorf("notConfigKeys lists %q, which is not a Namespace constant any more: remove it", v)
		}
		if slices.Contains(known, v) {
			t.Errorf("%q is both known and listed as not a config key", v)
		}
	}
}

// Every alias must lead to a name the code reads, and an alias must not be taken twice.
func TestKnownNamespaces_aliasesAreConsistent(t *testing.T) {
	seen := map[string]string{}
	for long, alias := range aliases {
		if other, dup := seen[alias]; dup {
			t.Errorf("alias %q is used for both %q and %q", alias, other, long)
		}
		seen[alias] = long
		if alias == long {
			t.Errorf("alias %q is the same as its code name", alias)
		}
	}
	if !slices.IsSorted(KnownNamespaces()) {
		t.Error("KnownNamespaces must be sorted, so suggestions and messages are stable")
	}
	for _, want := range []string{"router/stream", "inkgate/engine/router/stream", "qos/ratelimit/router", "telemetry/opentelemetry"} {
		if !slices.Contains(KnownNamespaces(), want) {
			t.Errorf("%q must be known", want)
		}
	}
}

// The JSON schema lists the namespaces at each level. It must list exactly the ones the gateway reads, so `check -n`
// and `check` agree.
func TestSchema_listsTheKnownNamespaces(t *testing.T) {
	var doc struct {
		Defs map[string]struct {
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal([]byte(schema.JSON), &doc); err != nil {
		t.Fatal(err)
	}
	inSchema := map[string]bool{}
	for _, level := range []string{"extraConfigService", "extraConfigEndpoint", "extraConfigBackend", "extraConfigAgent"} {
		def, ok := doc.Defs[level]
		if !ok || len(def.Properties) == 0 {
			t.Fatalf("schema has no %s", level)
		}
		for ns := range def.Properties {
			inSchema[ns] = true
		}
	}
	for _, ns := range KnownNamespaces() {
		if !inSchema[ns] {
			t.Errorf("%q is known to the gateway but not in schema.json", ns)
		}
	}
	for ns := range inSchema {
		if !slices.Contains(KnownNamespaces(), ns) {
			t.Errorf("%q is in schema.json but the gateway does not read it", ns)
		}
	}
}
