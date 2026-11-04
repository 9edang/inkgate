package audit

import (
	"testing"

	router "github.com/9edang/inkgate/engine/router"
	server "github.com/9edang/inkgate/engine/transport/http/server/plugin"
	jose "github.com/9edang/inkgate/feature/auth/jose"
	botdetector "github.com/9edang/inkgate/feature/security/botdetector/gateway"
	cors "github.com/9edang/inkgate/feature/security/cors"
	httpsecure "github.com/9edang/inkgate/feature/security/httpsecure"
	gelf "github.com/9edang/inkgate/feature/telemetry/gelf"
	gologging "github.com/9edang/inkgate/feature/telemetry/gologging"
	logstash "github.com/9edang/inkgate/feature/telemetry/logstash"
	metrics "github.com/9edang/inkgate/feature/telemetry/metrics"
	opencensus "github.com/9edang/inkgate/feature/telemetry/opencensus"
	cb "github.com/9edang/inkgate/feature/traffic/circuitbreaker/gobreaker"
	ratelimitProxy "github.com/9edang/inkgate/feature/traffic/ratelimit/proxy"
	ratelimit "github.com/9edang/inkgate/feature/traffic/ratelimit/router"
)

func Test_hasBasicAuth(t *testing.T) {
	if !hasBasicAuth(&Service{Components: Component{server.Namespace: []int{4}}}) {
		t.Error("false negative")
	}

	if hasBasicAuth(&Service{Components: Component{}}) {
		t.Error("false positive")
	}

	if hasBasicAuth(&Service{Components: Component{server.Namespace: []int{0}}}) {
		t.Error("false positive")
	}
}

func Test_hasApiKeys(t *testing.T) {
	if !hasApiKeys(&Service{Components: Component{"auth/api-keys": []int{}}}) {
		t.Error("false negative")
	}

	if hasApiKeys(&Service{Components: Component{}}) {
		t.Error("false positive")
	}
}

func Test_hasNoJWT(t *testing.T) {
	if hasNoJWT(&Service{Endpoints: []Endpoint{{Components: Component{jose.ValidatorNamespace: []int{}}}}}) {
		t.Error("false positive")
	}

	if !hasNoJWT(&Service{Components: Component{}}) {
		t.Error("false negative")
	}
}

func Test_hasInsecureConnections(t *testing.T) {
	if hasInsecureConnections(&Service{Details: []int{2}}) {
		t.Error("false positive")
	}

	if !hasInsecureConnections(&Service{Details: []int{24}}) {
		t.Error("false negative")
	}
}

func Test_hasNoTLS(t *testing.T) {
	if hasNoTLS(&Service{Details: []int{1 << 5}}) {
		t.Error("false positive")
	}

	if !hasNoTLS(&Service{Details: []int{24}}) {
		t.Error("false negative")
	}
}

func Test_hasTLSDisabled(t *testing.T) {
	if hasTLSDisabled(&Service{Details: []int{1 << 6}}) {
		t.Error("false positive")
	}
	if hasTLSDisabled(&Service{Details: []int{1<<5 + 1<<6}}) {
		t.Error("false positive")
	}

	if !hasTLSDisabled(&Service{Details: []int{1 << 5}}) {
		t.Error("false negative")
	}
}

func Test_hasNoHTTPSecure(t *testing.T) {
	if hasNoHTTPSecure(&Service{Components: Component{httpsecure.Namespace: []int{}}}) {
		t.Error("false positive")
	}

	if !hasNoHTTPSecure(&Service{Components: Component{}}) {
		t.Error("false negative")
	}
}

func Test_hasNoObfuscatedVersionHeader(t *testing.T) {
	if hasNoObfuscatedVersionHeader(&Service{Components: Component{router.Namespace: []int{1 << 17}}}) {
		t.Error("false positive")
	}

	if !hasNoObfuscatedVersionHeader(&Service{Components: Component{}}) {
		t.Error("false negative")
	}
}

func Test_hasNoCORS(t *testing.T) {
	if hasNoCORS(&Service{Components: Component{cors.Namespace: []int{1 << 17}}}) {
		t.Error("false positive")
	}

	if !hasNoCORS(&Service{Components: Component{}}) {
		t.Error("false negative")
	}
}

func Test_hasBotdetectorDisabled(t *testing.T) {
	if hasBotdetectorDisabled(&Service{Components: Component{botdetector.Namespace: []int{1 << 17}}}) {
		t.Error("false positive")
	}

	if !hasBotdetectorDisabled(&Service{Components: Component{}}) {
		t.Error("false negative")
	}
}

func Test_hasNoRatelimit(t *testing.T) {
	if hasNoRatelimit(&Service{Components: Component{ratelimit.Namespace: []int{1 << 17}}}) {
		t.Error("false positive")
	}
	if hasNoRatelimit(&Service{Endpoints: []Endpoint{{Components: Component{ratelimit.Namespace: []int{1 << 17}}}}}) {
		t.Error("false positive")
	}
	if hasNoRatelimit(&Service{Endpoints: []Endpoint{{Components: Component{ratelimitProxy.Namespace: []int{1 << 17}}}}}) {
		t.Error("false positive")
	}
	if hasNoRatelimit(&Service{Endpoints: []Endpoint{{Backends: []Backend{{Components: Component{ratelimitProxy.Namespace: []int{1 << 17}}}}}}}) {
		t.Error("false positive")
	}

	if !hasNoRatelimit(&Service{Components: Component{}}) {
		t.Error("false negative")
	}
}

func Test_hasNoCB(t *testing.T) {
	if hasNoCB(&Service{Endpoints: []Endpoint{{Components: Component{cb.Namespace: []int{1 << 17}}}}}) {
		t.Error("false positive")
	}
	if hasNoCB(&Service{Endpoints: []Endpoint{{Backends: []Backend{{Components: Component{cb.Namespace: []int{1 << 17}}}}}}}) {
		t.Error("false positive")
	}

	if !hasNoCB(&Service{Components: Component{}}) {
		t.Error("false negative")
	}
}

func Test_hasTimeoutBiggerThan(t *testing.T) {
	if hasTimeoutBiggerThan(1000)(&Service{Endpoints: []Endpoint{{Details: []int{0, 0, 0, 100}}}}) {
		t.Error("false positive")
	}

	if !hasTimeoutBiggerThan(1000)(&Service{Endpoints: []Endpoint{{Details: []int{0, 0, 0, 10000}}}}) {
		t.Error("false negative")
	}
}

func Test_hasNoMetrics(t *testing.T) {
	if hasNoMetrics(&Service{Components: Component{opencensus.Namespace: []int{1 << 17}}}) {
		t.Error("false positive")
	}
	if hasNoMetrics(&Service{Components: Component{metrics.Namespace: []int{1 << 17}}}) {
		t.Error("false positive")
	}
	if hasNoMetrics(&Service{Components: Component{"telemetry/newrelic": []int{1 << 17}}}) {
		t.Error("false positive")
	}
	if hasNoMetrics(&Service{Components: Component{"telemetry/ganalytics": []int{1 << 17}}}) {
		t.Error("false positive")
	}
	if hasNoMetrics(&Service{Components: Component{"telemetry/instana": []int{1 << 17}}}) {
		t.Error("false positive")
	}

	if !hasNoMetrics(&Service{Components: Component{}}) {
		t.Error("false negative")
	}
}

func Test_hasSeveralTelemetryComponents(t *testing.T) {
	if hasSeveralTelemetryComponents(&Service{Components: Component{opencensus.Namespace: []int{1 << 17}}}) {
		t.Error("false positive")
	}
	if hasSeveralTelemetryComponents(&Service{Components: Component{metrics.Namespace: []int{1 << 17}}}) {
		t.Error("false positive")
	}
	if hasSeveralTelemetryComponents(&Service{Components: Component{"telemetry/newrelic": []int{1 << 17}}}) {
		t.Error("false positive")
	}
	if hasSeveralTelemetryComponents(&Service{Components: Component{"telemetry/ganalytics": []int{1 << 17}}}) {
		t.Error("false positive")
	}
	if hasSeveralTelemetryComponents(&Service{Components: Component{"telemetry/instana": []int{1 << 17}}}) {
		t.Error("false positive")
	}
	if hasSeveralTelemetryComponents(&Service{Components: Component{}}) {
		t.Error("false positive")
	}

	if !hasSeveralTelemetryComponents(&Service{Components: Component{
		opencensus.Namespace: []int{1 << 17},
		metrics.Namespace:    []int{1 << 17},
	}}) {
		t.Error("false negative")
	}
}

func Test_hasNoTracing(t *testing.T) {
	if hasNoTracing(&Service{Components: Component{opencensus.Namespace: []int{1 << 17}}}) {
		t.Error("false positive")
	}
	if hasNoTracing(&Service{Components: Component{"telemetry/newrelic": []int{1 << 17}}}) {
		t.Error("false positive")
	}
	if hasNoTracing(&Service{Components: Component{"telemetry/instana": []int{1 << 17}}}) {
		t.Error("false positive")
	}

	if !hasNoTracing(&Service{Components: Component{}}) {
		t.Error("false negative")
	}
}

func Test_hasNoLogging(t *testing.T) {
	if hasNoLogging(&Service{Components: Component{gologging.Namespace: []int{1 << 17}}}) {
		t.Error("false positive")
	}
	if hasNoLogging(&Service{Components: Component{gelf.Namespace: []int{1 << 17}}}) {
		t.Error("false positive")
	}
	if hasNoLogging(&Service{Components: Component{logstash.Namespace: []int{1 << 17}}}) {
		t.Error("false positive")
	}

	if !hasNoLogging(&Service{Components: Component{}}) {
		t.Error("false negative")
	}
}

func Test_hasRestfulDisabled(t *testing.T) {
	if hasRestfulDisabled(&Service{Details: []int{0}}) {
		t.Error("false positive")
	}

	if !hasRestfulDisabled(&Service{Details: []int{1 << ServiceDisableStrictREST}}) {
		t.Error("false negative")
	}
}

func Test_hasDebugEnabled(t *testing.T) {
	if hasDebugEnabled(&Service{Details: []int{0}}) {
		t.Error("false positive")
	}

	if !hasDebugEnabled(&Service{Details: []int{1 << ServiceDebug}}) {
		t.Error("false negative")
	}
}

func Test_hasEndpointWithoutBackends(t *testing.T) {
	if hasEndpointWithoutBackends(&Service{Endpoints: []Endpoint{{Backends: []Backend{{}}}}}) {
		t.Error("false positive")
	}

	if !hasEndpointWithoutBackends(&Service{Endpoints: []Endpoint{{}}}) {
		t.Error("false negative")
	}
}

func Test_hasASingleBackendPerEndpoint(t *testing.T) {
	if hasASingleBackendPerEndpoint(&Service{Endpoints: []Endpoint{{Backends: []Backend{{}, {}}}}}) {
		t.Error("false positive")
	}

	if !hasASingleBackendPerEndpoint(&Service{Endpoints: []Endpoint{{Backends: []Backend{{}}}}}) {
		t.Error("false negative")
	}
}

func Test_hasAllEndpointsAsNoop(t *testing.T) {
	if hasAllEndpointsAsNoop(&Service{Endpoints: []Endpoint{{Details: []int{2}}}}) {
		t.Error("false positive")
	}

	if !hasAllEndpointsAsNoop(&Service{Endpoints: []Endpoint{{Details: []int{1}}}}) {
		t.Error("false negative")
	}
}
