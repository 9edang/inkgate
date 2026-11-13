package lambda

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
)

// fakeLambda answers the Invoke API of AWS Lambda like the real service: POST /2015-03-31/functions/<name>/invocations.
// The first `failures` calls answer 500, which the SDK retries.
func fakeLambda(t *testing.T, failures int32) (*httptest.Server, *atomic.Int32, *atomic.Value) {
	t.Helper()
	var calls atomic.Int32
	var lastPath atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		body, _ := io.ReadAll(r.Body)
		if len(body) == 0 {
			body = []byte("null")
		}
		lastPath.Store(r.URL.Path)
		if n <= failures {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Amz-Executed-Version", "$LATEST")
		_, _ = w.Write([]byte(`{"message":"hello","echo":` + string(body) + `}`))
	}))
	t.Cleanup(srv.Close)

	return srv, &calls, &lastPath
}

func awsEnv(t *testing.T) {
	t.Helper()
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
	t.Setenv("AWS_CONFIG_FILE", "/dev/null")
	t.Setenv("AWS_SHARED_CREDENTIALS_FILE", "/dev/null")
}

func callBackend(t *testing.T, extra map[string]any) (*proxy.Response, error) {
	t.Helper()
	bf := BackendFactory(logging.NoOp, func(*config.Backend) proxy.Proxy {
		t.Fatal("the lambda backend must handle the call")
		return nil
	})
	p := bf(&config.Backend{
		Method:      "POST",
		URLPattern:  "/fn",
		ExtraConfig: config.ExtraConfig{Namespace: extra},
	})

	return p(context.Background(), &proxy.Request{Body: io.NopCloser(strings.NewReader(`{"a":1}`))})
}

// The real SDK client, pointed at a local endpoint: function name, payload, status and executed version.
func TestInvokerFactory_callsTheEndpointOfTheConfig(t *testing.T) {
	awsEnv(t)
	srv, calls, path := fakeLambda(t, 0)

	resp, err := callBackend(t, map[string]any{"function_name": "my-fn", "region": "eu-west-1", "endpoint": srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || path.Load() != "/2015-03-31/functions/my-fn/invocations" {
		t.Errorf("calls %d, path %v", calls.Load(), path.Load())
	}
	if resp.Data["message"] != "hello" || resp.Metadata.Headers["X-Amz-Executed-Version"][0] != "$LATEST" {
		t.Errorf("unexpected response %+v", resp)
	}
}

// max_retries is a JSON number. It must take effect: with 1 retry a call that fails once succeeds (2 attempts), with none it fails.
func TestInvokerFactory_maxRetries(t *testing.T) {
	awsEnv(t)
	for name, tc := range map[string]struct {
		retries  any
		wantErr  bool
		wantHits int32
	}{
		"no retry":     {retries: 0.0, wantErr: true, wantHits: 1},
		"one retry":    {retries: 1.0, wantErr: false, wantHits: 2},
		"not a number": {retries: "1", wantErr: false, wantHits: 2}, // ignored: the SDK default (3 attempts) recovers
		"negative":     {retries: -1.0, wantErr: false, wantHits: 2},
		"not whole":    {retries: 1.5, wantErr: false, wantHits: 2},
	} {
		srv, calls, _ := fakeLambda(t, 1)
		_, err := callBackend(t, map[string]any{"function_name": "f", "region": "us-east-1", "endpoint": srv.URL, "max_retries": tc.retries})
		if (err != nil) != tc.wantErr {
			t.Errorf("%s: error %v, want error %v", name, err, tc.wantErr)
		}
		if calls.Load() != tc.wantHits {
			t.Errorf("%s: %d calls, want %d", name, calls.Load(), tc.wantHits)
		}
	}
}

// Without a region in the config it comes from the AWS environment; the endpoint still applies.
func TestInvokerFactory_regionFromTheEnvironment(t *testing.T) {
	awsEnv(t)
	t.Setenv("AWS_REGION", "ap-southeast-2")
	srv, calls, _ := fakeLambda(t, 0)
	if _, err := callBackend(t, map[string]any{"function_name": "f", "endpoint": srv.URL}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 {
		t.Errorf("%d calls, want 1", calls.Load())
	}
}

// A backend whose AWS client cannot be built fails its own requests, not the gateway.
func TestFailedInvoker(t *testing.T) {
	_, err := failedInvoker{err: io.ErrUnexpectedEOF}.Invoke(context.Background(), nil)
	if err == nil {
		t.Error("a failed invoker must return its error")
	}
}
