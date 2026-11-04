package audit

import (
	"testing"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/encoding"
	router "github.com/9edang/inkgate/engine/router"
)

func TestParse(t *testing.T) {
	cfg, err := config.NewParser().Parse("./tests/example1.json")
	if err != nil {
		t.Error(err.Error())
	}
	cfg.AllowInsecureConnections = true
	cfg.TLS.EnableMTLS = true
	cfg.TLS.DisableSystemCaPool = true
	cfg.TLS.CaCerts = []string{"path/to/cacert"}
	cfg.ExtraConfig[router.Namespace] = map[string]any{
		"error_body":                        true,
		"disable_health":                    true,
		"disable_access_log":                true,
		"health_path":                       "/health",
		"return_error_msg":                  true,
		"disable_redirect_trailing_slash":   true,
		"disable_redirect_fixed_path":       true,
		"remove_extra_slash":                true,
		"disable_handle_method_not_allowed": true,
		"disable_path_decoding":             true,
		"auto_options":                      true,
		"forwarded_by_client_ip":            true,
		"remote_ip_headers":                 []any{"x-header-a"},
		"trusted_proxies":                   []any{"1.1.1.1"},
		"app_engine":                        true,
		"max_multipart_memory":              10.0,
		"logger_skip_paths":                 []any{"/health"},
		"hide_version_header":               true,
	}
	cfg.Endpoints[0].OutputEncoding = encoding.SAFE_JSON
	cfg.Endpoints[0].Backend[0].Target = "foo"
	cfg.Endpoints[0].Backend[0].IsCollection = true
	cfg.Normalize()

	result := Parse(&cfg)

	if len(result.Endpoints) != len(cfg.Endpoints) {
		t.Errorf("unexpected number of endpoints. have: %d, want: %d", len(result.Endpoints), len(cfg.Endpoints))
	}

	if len(result.Agents) != len(cfg.AsyncAgents) {
		t.Errorf("unexpected number of agents. have: %d, want: %d", len(result.Agents), len(cfg.AsyncAgents))
	}

	if len(result.Details) != 1 {
		t.Errorf("unexpected number of details. have: %d, want: 1", len(result.Details))
		return
	}

	if result.Details[0] != 8124 {
		t.Errorf("unexpected service details. have: %d, want: 4028", result.Details[0])
	}

	if len(result.Endpoints[0].Details) != 6 {
		t.Errorf("unexpected number of endpoint details. have: %d, want: 5", len(result.Endpoints[0].Details))
		return
	}

	for i, v := range []int{4, 0, 0, 140000} {
		if result.Endpoints[0].Details[i] != v {
			t.Errorf("unexpected endpoint details. have: %d, want: %d", result.Endpoints[0].Details[i], v)
		}
	}

	if len(result.Endpoints[0].Backends[0].Details) != 1 {
		t.Errorf("unexpected number of backend details. have: %d, want: 1", len(result.Endpoints[0].Backends[0].Details))
		return
	}

	if result.Endpoints[0].Backends[0].Details[0] != 6208 {
		t.Errorf("unexpected backend details. have: %d, want: 6208", result.Endpoints[0].Backends[0].Details[0])
	}
}
