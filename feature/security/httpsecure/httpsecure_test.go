package httpsecure

import (
	"fmt"
	"github.com/9edang/inkgate/internal/json"

	"github.com/9edang/inkgate/engine/config"
)

func ExampleConfigGetter() {
	cfg := ConfigGetter(config.ExtraConfig{
		Namespace: map[string]any{
			"allowed_hosts":      []any{"host1"},
			"host_proxy_headers": []any{"x-custom-header"},
			"sts_seconds":        10.0,
			"ssl_redirect":       true,
			"ssl_host":           "secure.example.com",
		},
	})
	fmt.Println(cfg)

	// output:
	// {false false false false false false true false false false false         secure.example.com [host1] false <nil> [x-custom-header] <nil> map[] 10 }
}

func ExampleConfigGetter_fromParsedData() {
	sample := `{
            "allowed_hosts": ["host1"],
            "ssl_proxy_headers": {},
            "sts_seconds": 300,
            "frame_deny": true,
            "sts_include_subdomains": true
        }`
	parsedCfg := map[string]any{}

	if err := json.Unmarshal([]byte(sample), &parsedCfg); err != nil {
		fmt.Println(err)
		return
	}

	cfg := ConfigGetter(config.ExtraConfig{Namespace: parsedCfg})
	fmt.Println(cfg)

	// output:
	// {false false false true false false false false false true false          [host1] false <nil> [] <nil> map[] 300 }
}
