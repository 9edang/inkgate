// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"errors"
	"testing"

	"github.com/9edang/inkgate/engine/config"
)

func withValidationState(t *testing.T, known []string, allow bool, custom func(config.ServiceConfig) []error) {
	t.Helper()
	oldKnown, oldAllow, oldCustom := KnownNamespaces, allowUnknownNamespaces, CustomValidationFunc
	t.Cleanup(func() { KnownNamespaces, allowUnknownNamespaces, CustomValidationFunc = oldKnown, oldAllow, oldCustom })
	KnownNamespaces, allowUnknownNamespaces, CustomValidationFunc = known, allow, custom
}

func typoConfig() config.ServiceConfig {
	return config.ServiceConfig{Endpoints: []*config.EndpointConfig{{
		Endpoint: "/x", Method: "GET", ExtraConfig: config.ExtraConfig{"router/stram": map[string]any{}},
	}}}
}

func noCustom(config.ServiceConfig) []error { return nil }

func TestValidateConfig_refusesAnUnknownNamespaceAndSaysHowToGoOn(t *testing.T) {
	withValidationState(t, []string{"router/stream"}, false, noCustom)

	errs, hint := validateConfig(typoConfig())
	if len(errs) != 1 {
		t.Fatalf("want one error, got %v", errs)
	}
	var u *config.UnknownNamespaceError
	if !errors.As(errs[0], &u) || u.Namespace != "router/stram" || u.Suggestion != "router/stream" {
		t.Errorf("unexpected error %v", errs[0])
	}
	if hint == "" {
		t.Error("an unknown namespace comes with the hint about --allow-unknown-namespaces")
	}
}

func TestValidateConfig_theFlagAcceptsUnknownNamespacesButNotOtherErrors(t *testing.T) {
	custom := errors.New("bad option")
	withValidationState(t, []string{"router/stream"}, true, func(config.ServiceConfig) []error { return []error{custom} })

	errs, hint := validateConfig(typoConfig())
	if len(errs) != 1 || !errors.Is(errs[0], custom) {
		t.Fatalf("with the flag only the custom validation remains, got %v", errs)
	}
	if hint != "" {
		t.Errorf("no hint without namespace errors, got %q", hint)
	}
}

func TestValidateConfig_withoutKnownNamespacesTheCheckIsOff(t *testing.T) {
	withValidationState(t, nil, false, noCustom)

	if errs, _ := validateConfig(typoConfig()); len(errs) != 0 {
		t.Fatalf("a build that registers no namespaces cannot judge them: %v", errs)
	}
}

func TestValidateConfig_customErrorsComeAfterNamespaceErrors(t *testing.T) {
	custom := errors.New("stream option")
	withValidationState(t, []string{"router/stream"}, false, func(config.ServiceConfig) []error { return []error{custom} })

	errs, _ := validateConfig(typoConfig())
	if len(errs) != 2 || !errors.Is(errs[1], custom) {
		t.Fatalf("want the namespace error, then the custom one, got %v", errs)
	}
}
