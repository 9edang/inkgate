package config

import "encoding/json"

// Float64 reads a number from a decoded extra_config value, whatever decoder produced it: encoding/json and koanf
// give float64, internal/json (sonic, JSON_USE_INT64) gives int64 for whole numbers, and UseNumber gives
// json.Number. A type assertion to float64 alone silently ignores the setting under the other decoders.
func Float64(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case int:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default: // a quoted "200" is not a number and stays a config mistake
		return 0, false
	}
}
