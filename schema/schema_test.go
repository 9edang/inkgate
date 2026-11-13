// SPDX-License-Identifier: Apache-2.0

package schema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// compile does what `inkgate check --lint-no-network` does with the embedded schema.
func compile(t *testing.T) *jsonschema.Schema {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(JSON))
	if err != nil {
		t.Fatalf("schema.json is not JSON: %v", err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", doc); err != nil {
		t.Fatal(err)
	}
	sch, err := c.Compile("schema.json")
	if err != nil {
		t.Fatalf("schema.json does not compile: %v", err)
	}

	return sch
}

func validate(t *testing.T, sch *jsonschema.Schema, doc map[string]any) error {
	t.Helper()
	// through JSON, so numbers and nesting are what the file loader gives
	raw, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	inst, err := jsonschema.UnmarshalJSON(strings.NewReader(string(raw)))
	if err != nil {
		t.Fatal(err)
	}

	return sch.Validate(inst)
}

// The configs of this repository are real configs: the schema must accept every one.
func TestSchema_acceptsTheConfigsOfTheRepository(t *testing.T) {
	sch := compile(t)
	// tests/fixtures/koanf/ok.json is left out: it tests the parser with arbitrary keys, it is not a gateway config
	files := []string{"../inkgate.json", "../tests/fixtures/inkgate.json", "../tests/fixtures/bench.json"}
	bench, err := filepath.Glob("../benchmark/config/*.json")
	if err != nil || len(bench) < 4 {
		t.Fatalf("benchmark configs not found: %v %v", bench, err)
	}
	files = append(files, bench...)
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		inst, err := jsonschema.UnmarshalJSON(strings.NewReader(string(raw)))
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if err := sch.Validate(inst); err != nil {
			t.Errorf("%s is refused:\n%v", f, err)
		}
	}
}

func baseConfig() map[string]any {
	return map[string]any{
		"version": 3,
		"port":    8080,
		"timeout": "3s",
		"endpoints": []any{map[string]any{
			"endpoint": "/users/{id}",
			"method":   "GET",
			"backend":  []any{map[string]any{"host": []any{"http://users:8000"}, "url_pattern": "/users/{id}"}},
		}},
	}
}

func endpoint(doc map[string]any) map[string]any {
	return doc["endpoints"].([]any)[0].(map[string]any)
}

func backend(doc map[string]any) map[string]any {
	return endpoint(doc)["backend"].([]any)[0].(map[string]any)
}

// Each case changes one thing of a valid config. The change makes the config bad, and the same place filled in
// correctly stays valid, so a refusal is about that change and not about something else in the document.
func TestSchema_refusesWhatTheGatewayWouldIgnore(t *testing.T) {
	sch := compile(t)
	if err := validate(t, sch, baseConfig()); err != nil {
		t.Fatalf("the base config must be valid: %v", err)
	}
	type change func(doc map[string]any)
	setExtra := func(level string, ns string, v any) change {
		return func(doc map[string]any) {
			var holder map[string]any
			switch level {
			case "service":
				holder = doc
			case "endpoint":
				holder = endpoint(doc)
			case "backend":
				holder = backend(doc)
			}
			ec, _ := holder["extra_config"].(map[string]any)
			if ec == nil {
				ec = map[string]any{}
				holder["extra_config"] = ec
			}
			ec[ns] = v
		}
	}
	set := func(level, key string, v any) change {
		return func(doc map[string]any) {
			switch level {
			case "service":
				doc[key] = v
			case "endpoint":
				endpoint(doc)[key] = v
			case "backend":
				backend(doc)[key] = v
			}
		}
	}
	stream := func(k string, v any) change {
		return setExtra("endpoint", "router/stream", map[string]any{"mode": "socket", k: v})
	}

	for name, tc := range map[string]struct{ good, bad change }{
		"a service key":                {set("service", "read_timeout", "30s"), set("service", "read_timout", "30s")},
		"an endpoint key":              {set("endpoint", "output_encoding", "json"), set("endpoint", "ouput_encoding", "json")},
		"a backend key":                {set("backend", "is_collection", true), set("backend", "is_colection", true)},
		"a namespace name":             {setExtra("service", "telemetry/logging", map[string]any{"level": "INFO"}), setExtra("service", "telemetry/loging", map[string]any{"level": "INFO"})},
		"a KrakenD namespace":          {setExtra("endpoint", "qos/ratelimit/router", map[string]any{"max_rate": 10}), setExtra("endpoint", "github_com/devopsfaith/krakend-ratelimit/juju/router", map[string]any{"max_rate": 10})},
		"a namespace at its own level": {setExtra("endpoint", "router/stream", map[string]any{"mode": "event"}), setExtra("service", "router/stream", map[string]any{"mode": "event"})},
		"a stream option":              {stream("max_connections", 10), stream("max_conections", 10)},
		"a rate limit option":          {setExtra("endpoint", "qos/ratelimit/router", map[string]any{"max_rate": 5}), setExtra("endpoint", "qos/ratelimit/router", map[string]any{"max_rte": 5})},
		"an option of a jwt validator": {setExtra("endpoint", "auth/validator", map[string]any{"alg": "RS256", "jwk_url": "https://x/jwks"}), setExtra("endpoint", "auth/validator", map[string]any{"alg": "RS256", "jwk_ulr": "https://x/jwks"})},
		"a duration":                   {set("service", "timeout", "3s"), set("service", "timeout", 3000)},
		"a negative duration":          {set("service", "read_header_timeout", "-1s"), set("service", "read_header_timeout", "soon")},
		"a port":                       {set("service", "port", 8080), set("service", "port", "8080")},
		"an enum":                      {setExtra("endpoint", "router/stream", map[string]any{"mode": "socket"}), setExtra("endpoint", "router/stream", map[string]any{"mode": "websocket"})},
		"the output encoding":          {set("endpoint", "output_encoding", "no-op"), set("endpoint", "output_encoding", "jsno")},
		"the code name of a namespace": {setExtra("endpoint", "inkgate/engine/router/stream", map[string]any{"mode": "event"}), setExtra("endpoint", "inkgate/engine/router/strem", map[string]any{"mode": "event"})},
		"a stream value":               {setExtra("endpoint", "router/stream", map[string]any{}), setExtra("endpoint", "router/stream", true)},
		"a burst without a rate":       {stream("max_messages_per_second", 10), stream("message_burst", 10)},
		"the plugin names":             {setExtra("service", "plugin/http-server", map[string]any{"name": []any{"p"}, "p": map[string]any{"anything": 1}}), setExtra("service", "plugin/http-server", map[string]any{"name": 7})},
		"an opentelemetry option":      {setExtra("service", "telemetry/opentelemetry", map[string]any{"layers": map[string]any{"global": map[string]any{"disable_metrics": true}}}), setExtra("service", "telemetry/opentelemetry", map[string]any{"layers": map[string]any{"global": map[string]any{"disable_metric": true}}})},
	} {
		good, bad := baseConfig(), baseConfig()
		tc.good(good)
		tc.bad(bad)
		if err := validate(t, sch, good); err != nil {
			t.Errorf("%s: the valid form is refused: %v", name, err)
		}
		if err := validate(t, sch, bad); err == nil {
			t.Errorf("%s: the bad form is accepted, the gateway would ignore it silently", name)
		}
	}

	// required parts
	noVersion := baseConfig()
	delete(noVersion, "version")
	if validate(t, sch, noVersion) == nil {
		t.Error("a config without version must be refused")
	}
	noBackend := baseConfig()
	delete(endpoint(noBackend), "backend")
	if validate(t, sch, noBackend) == nil {
		t.Error("an endpoint without backends must be refused")
	}
	noURL := baseConfig()
	delete(backend(noURL), "url_pattern")
	if validate(t, sch, noURL) == nil {
		t.Error("a backend without url_pattern must be refused")
	}
}

// The examples of the docs are what people copy: a complete config, or an endpoint, in a json block, must pass.
func TestSchema_acceptsTheExamplesOfTheDocs(t *testing.T) {
	sch := compile(t)
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(JSON))
	if err != nil {
		t.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", doc); err != nil {
		t.Fatal(err)
	}
	endpointSchema, err := c.Compile("schema.json#/$defs/endpoint")
	if err != nil {
		t.Fatal(err)
	}

	var files []string
	err = filepath.WalkDir("..", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "results") {
			return filepath.SkipDir
		}
		if strings.HasSuffix(path, ".md") {
			files = append(files, path)
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	checked := 0
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for i, block := range jsonBlocks(string(raw)) {
			var probe map[string]any
			if json.Unmarshal([]byte(block), &probe) != nil {
				continue // a fragment, or JSON with "..." in it
			}
			var target *jsonschema.Schema
			switch _, hasVersion := probe["version"]; {
			case hasVersion && probe["endpoints"] != nil:
				target = sch
			case probe["endpoint"] != nil && probe["backend"] != nil:
				target = endpointSchema
			default:
				continue
			}
			inst, err := jsonschema.UnmarshalJSON(strings.NewReader(block))
			if err != nil {
				continue
			}
			checked++
			if err := target.Validate(inst); err != nil {
				t.Errorf("%s, json block %d is refused:\n%v", f, i+1, err)
			}
		}
	}
	if checked < 3 {
		t.Fatalf("found only %d config examples in the docs: the extraction is broken", checked)
	}
}

// jsonBlocks returns the content of the fenced blocks of a Markdown text that may hold JSON: the ones marked json and
// the ones without a language. What is not JSON is skipped by the caller.
func jsonBlocks(md string) []string {
	var blocks []string
	var cur []string
	in, other := false, false
	for _, line := range strings.Split(md, "\n") {
		trimmed := strings.TrimSpace(line)
		fence := strings.HasPrefix(trimmed, "```")
		switch {
		case fence && !in && !other:
			if lang := strings.TrimPrefix(trimmed, "```"); lang == "" || lang == "json" {
				in, cur = true, nil
			} else {
				other = true
			}
		case fence && in:
			in = false
			blocks = append(blocks, strings.Join(cur, "\n"))
		case fence && other:
			other = false
		case in:
			cur = append(cur, line)
		}
	}

	return blocks
}
