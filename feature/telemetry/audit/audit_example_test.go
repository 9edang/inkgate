package audit

import (
	"fmt"

	"github.com/9edang/inkgate/engine/config"
)

func ExampleAudit() {
	cfg, err := config.NewParser().Parse("../../../tests/fixtures/audit/example1.json")
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	cfg.Normalize()

	exclude := []string{"1.1.1", "1.1.2", "7.2.4"}
	levels := []string{SeverityCritical, SeverityHigh, SeverityMedium}

	result, err := Audit(&cfg, exclude, levels)
	if err != nil {
		fmt.Println(err)
		return
	}

	for i, r := range result.Recommendations {
		fmt.Printf("%02d: %s %s  \t%s\n", i, r.Rule, r.Severity, r.Message)
	}

	// output:
	// 00: 2.1.3 CRITICAL  	TLS is configured but its disable flag prevents from using it.
	// 01: 2.1.7 HIGH  	Enable HTTP security header checks (security/http).
	// 02: 2.1.8 HIGH  	Avoid clear text communication (h2c).
	// 03: 2.2.1 MEDIUM  	Hide the version banner in runtime.
	// 04: 2.2.2 HIGH  	Enable CORS.
	// 05: 2.2.3 HIGH  	Avoid passing all input headers to the backend.
	// 06: 2.2.4 HIGH  	Avoid passing all input query strings to the backend.
	// 07: 2.3.1 MEDIUM  	Limit the amount of cacheable content.
	// 08: 3.1.3 HIGH  	Protect your backends with a circuit breaker.
	// 09: 3.3.2 MEDIUM  	Set timeouts to below 5 seconds for improved performance.
	// 10: 3.3.3 HIGH  	Set timeouts to below 30 seconds for improved performance.
	// 11: 3.3.4 CRITICAL  	Set timeouts to below 1 minute for improved performance.
	// 12: 4.1.1 MEDIUM  	Implement a telemetry system for collecting metrics for monitoring and troubleshooting.
	// 13: 4.1.3 HIGH  	Avoid duplicating telemetry options to prevent system overload.
	// 14: 4.3.1 MEDIUM  	Use the improved logging component for better log parsing.
	// 15: 5.1.5 MEDIUM  	Declare explicit endpoints instead of using /__catchall.
	// 16: 5.1.6 MEDIUM  	Avoid using multiple write methods in endpoint definitions.
	// 17: 5.1.7 MEDIUM  	Avoid using sequential proxy.
	// 18: 7.1.3 HIGH  	Avoid using deprecated plugin basic-auth. Please move your configuration to the namespace auth/basic to use the new component.
	// 19: 7.1.7 HIGH  	Avoid using deprecated plugin no-redirect.
	// 20: 7.3.1 MEDIUM  	Avoid using 'private_key' and 'public_key' and use the 'keys' array.
}
