package config

import (
	"encoding/json"
	"testing"
)

func TestFloat64(t *testing.T) {
	for _, tc := range []struct {
		in   any
		want float64
		ok   bool
	}{
		{float64(1.5), 1.5, true},
		{int64(200), 200, true},
		{int(7), 7, true},
		{json.Number("1048576"), 1048576, true},
		{json.Number("x"), 0, false},
		{"200", 0, false},
		{nil, 0, false},
		{true, 0, false},
	} {
		got, ok := Float64(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Errorf("Float64(%#v) = %v, %v; want %v, %v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
