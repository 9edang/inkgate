// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"testing"
)

var testKnown = []string{"router", "router/stream", "qos/ratelimit/router", "telemetry/logging", "inkgate/engine/router/stream"}

func unknown(t *testing.T, errs []error) []UnknownNamespaceError {
	t.Helper()
	var out []UnknownNamespaceError
	for _, err := range errs {
		var u *UnknownNamespaceError
		if !errors.As(err, &u) {
			t.Fatalf("unexpected error type %T: %v", err, err)
		}
		out = append(out, *u)
	}

	return out
}

func TestValidateNamespaces_namesWhereTheKeyIs(t *testing.T) {
	cfg := ServiceConfig{
		ExtraConfig: ExtraConfig{"router": map[string]any{}, "telemetry/loging": map[string]any{}},
		Endpoints: []*EndpointConfig{{
			Endpoint:    "/users",
			Method:      "GET",
			ExtraConfig: ExtraConfig{"router/stram": map[string]any{}, "router/stream": map[string]any{}},
			Backend: []*Backend{
				{URLPattern: "/a", ExtraConfig: ExtraConfig{"qos/ratelimit/router": map[string]any{}}},
				{URLPattern: "/b", ExtraConfig: ExtraConfig{"qos/ratelimt/router": map[string]any{}}},
			},
		}},
		AsyncAgents: []*AsyncAgent{{
			Name:        "agent1",
			ExtraConfig: ExtraConfig{"nothing/like/it": true},
			Backend:     []*Backend{{URLPattern: "/c", ExtraConfig: ExtraConfig{"router/stram": 1}}},
		}},
	}

	got := unknown(t, cfg.ValidateNamespaces(testKnown))
	want := []UnknownNamespaceError{
		{Where: "service", Namespace: "telemetry/loging", Suggestion: "telemetry/logging"},
		{Where: "endpoint GET /users", Namespace: "router/stram", Suggestion: "router/stream"},
		{Where: "endpoint GET /users, backend 2 (/b)", Namespace: "qos/ratelimt/router", Suggestion: "qos/ratelimit/router"},
		{Where: `async_agent "agent1"`, Namespace: "nothing/like/it"}, // nothing close: no suggestion
		{Where: `async_agent "agent1", backend 1 (/c)`, Namespace: "router/stram", Suggestion: "router/stream"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d errors %+v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("error %d: got %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestValidateNamespaces_validConfigHasNoErrors(t *testing.T) {
	cfg := ServiceConfig{
		ExtraConfig: ExtraConfig{"router": 1},
		Endpoints: []*EndpointConfig{{
			Endpoint:    "/x",
			ExtraConfig: ExtraConfig{"router/stream": 1, "inkgate/engine/router/stream": 2}, // alias and code name are both fine
		}, nil},
	}
	if errs := cfg.ValidateNamespaces(testKnown); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if errs := (&ServiceConfig{}).ValidateNamespaces(testKnown); len(errs) != 0 {
		t.Fatalf("an empty config has no unknown namespaces: %v", errs)
	}
}

func TestExtraConfigGet_findsAnAliasOrTheCodeName(t *testing.T) {
	defer func(old map[string]string) { ExtraConfigAlias = old }(ExtraConfigAlias)
	ExtraConfigAlias = map[string]string{"router/stream": "inkgate/engine/router/stream"}

	for name, e := range map[string]ExtraConfig{
		"by alias":     {"router/stream": "v"},
		"by code name": {"inkgate/engine/router/stream": "v"},
	} {
		if v, ok := e.Get("inkgate/engine/router/stream"); !ok || v != "v" {
			t.Errorf("%s: got %v, %v", name, v, ok)
		}
	}
	if _, ok := (ExtraConfig{"other": 1}).Get("inkgate/engine/router/stream"); ok {
		t.Error("found a namespace that is not there")
	}
	if _, ok := (ExtraConfig(nil)).Get("x"); ok {
		t.Error("a nil ExtraConfig has no namespaces")
	}
}

func TestSuggest(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"router/stram", "router/stream"},
		{"telemetry/loging", "telemetry/logging"},
		{"routr", "router"},
		{"rotuer", "router"}, // two letters swapped is one edit
		{"completely/different", ""},
		{"x", ""}, // short names are never a confident guess
		{"", ""},
	} {
		if got := Suggest(tc.name, testKnown); got != tc.want {
			t.Errorf("Suggest(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
	if got := Suggest("router", nil); got != "" {
		t.Errorf("no candidates must give no suggestion, got %q", got)
	}
}
