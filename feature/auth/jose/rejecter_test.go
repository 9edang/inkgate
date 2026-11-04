package jose

import (
	"testing"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
)

func TestChainedRejecterFactory(t *testing.T) {
	rf := ChainedRejecterFactory([]RejecterFactory{
		NopRejecterFactory{},
		RejecterFactoryFunc(func(_ logging.Logger, _ *config.EndpointConfig) Rejecter {
			return RejecterFunc(func(in map[string]any) bool {
				v, ok := in["key"].(int)
				return ok && v == 42
			})
		}),
	})

	rejecter := rf.New(nil, nil)

	for _, tc := range []struct {
		name     string
		in       map[string]any
		expected bool
	}{
		{
			name: "empty",
			in:   map[string]any{},
		},
		{
			name:     "reject",
			in:       map[string]any{"key": 42},
			expected: true,
		},
		{
			name: "pass-1",
			in:   map[string]any{"key": "42"},
		},
		{
			name: "pass-2",
			in:   map[string]any{"key": 9876},
		},
	} {
		if v := rejecter.Reject(tc.in); tc.expected != v {
			t.Errorf("unexpected result for %s. have %v, want %v", tc.name, v, tc.expected)
		}
	}
}
