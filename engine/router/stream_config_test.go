// SPDX-License-Identifier: Apache-2.0

package router

import (
	"strings"
	"testing"

	"github.com/9edang/inkgate/engine/config"
)

func streamEndpoint(key string, v any) *config.EndpointConfig {
	return &config.EndpointConfig{Endpoint: "/live", ExtraConfig: config.ExtraConfig{key: v}}
}

// "router/stream": true used to turn event mode on, and a value that is not an object was read as an empty one.
func TestValidateStream_refusesAValueThatIsNotAnObject(t *testing.T) {
	for name, v := range map[string]any{
		"bool":   true,
		"false":  false,
		"string": "event",
		"number": 3.0,
		"null":   nil,
		"list":   []any{"event"},
	} {
		err := ValidateStream(streamEndpoint(StreamNamespace, v))
		if err == nil {
			t.Errorf("%s: a section that is not an object must be refused", name)
			continue
		}
		if !strings.Contains(err.Error(), "/live") {
			t.Errorf("%s: the error must name the endpoint, got %q", name, err)
		}
	}
	if _, err := parseStream(streamEndpoint(StreamNamespace, true)); err == nil {
		t.Error("the router must refuse it too, not only the validation")
	}
}

func TestValidateStream_refusesAnUnknownOption(t *testing.T) {
	for name, tc := range map[string]struct {
		section map[string]any
		suggest string
	}{
		"typo":               {map[string]any{"mode": "socket", "max_conections": 10.0}, "max_connections"},
		"typo in the rate":   {map[string]any{"mode": "socket", "max_message_per_second": 5.0}, "max_messages_per_second"},
		"a made up option":   {map[string]any{"mode": "event", "compression": true}, ""},
		"even when disabled": {map[string]any{"enabled": false, "mdoe": "socket"}, "mode"},
	} {
		err := ValidateStream(streamEndpoint(StreamNamespace, tc.section))
		if err == nil {
			t.Errorf("%s: an unknown option must be refused, it would be ignored", name)
			continue
		}
		if !strings.Contains(err.Error(), "/live") {
			t.Errorf("%s: the error must name the endpoint, got %q", name, err)
		}
		if tc.suggest != "" && !strings.Contains(err.Error(), `"`+tc.suggest+`"`) {
			t.Errorf("%s: the error should suggest %q, got %q", name, tc.suggest, err)
		}
	}
}

func TestValidateStream_acceptsEveryDocumentedOption(t *testing.T) {
	socket := map[string]any{
		"mode": "socket", "idle_timeout": "60s", "max_message_size": 1024.0, "max_connections": 100.0,
		"allowed_origins": []any{"https://app.example.com"}, "subprotocols": []any{"chat"},
		"max_messages_per_second": 10.0, "message_burst": 20.0,
	}
	event := map[string]any{"mode": "event", "keep_alive": "15s", "max_connections": 5.0}
	for name, section := range map[string]map[string]any{
		"empty":    {},
		"disabled": {"enabled": false, "mode": "socket"},
		"socket":   socket,
		"event":    event,
	} {
		if err := ValidateStream(streamEndpoint(StreamNamespace, section)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	// the keys the code knows are exactly the ones the doc comment lists: adding one here means documenting it
	if len(streamKeys) != 10 {
		t.Errorf("streamKeys has %d options; document a new one in StreamNamespace and add it to this test", len(streamKeys))
	}
}

func TestValidateStream_findsTheSectionUnderItsAlias(t *testing.T) {
	defer func(old map[string]string) { config.ExtraConfigAlias = old }(config.ExtraConfigAlias)
	config.ExtraConfigAlias = map[string]string{"router/stream": StreamNamespace}

	if err := ValidateStream(streamEndpoint("router/stream", map[string]any{"mode": "event"})); err != nil {
		t.Fatalf("a valid section under its alias: %v", err)
	}
	if err := ValidateStream(streamEndpoint("router/stream", true)); err == nil {
		t.Fatal("a bad section under its alias must be refused before Normalize, as `check` sees it")
	}
	if err := ValidateStream(&config.EndpointConfig{Endpoint: "/x"}); err != nil {
		t.Fatalf("an endpoint that does not stream is fine: %v", err)
	}
}
