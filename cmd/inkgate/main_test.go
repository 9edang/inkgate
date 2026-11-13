// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestMain lets the tests below run the real program: the test binary starts itself as inkgate when asked to.
func TestMain(m *testing.M) {
	if os.Getenv("INKGATE_TEST_AS_MAIN") == "1" {
		os.Args = append([]string{"inkgate"}, strings.Fields(os.Getenv("INKGATE_TEST_ARGS"))...)
		main()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// inkgate runs `inkgate <args>` and returns its output and exit code (-1 when it had to be killed).
func inkgate(t *testing.T, args ...string) (string, int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), "INKGATE_TEST_AS_MAIN=1", "INKGATE_TEST_ARGS="+strings.Join(args, " "))
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return out.String(), 0
	case errors.As(err, &exit):
		return out.String(), exit.ExitCode()
	default:
		t.Fatalf("running inkgate: %v\n%s", err, out.String())
		return "", -1
	}
}

func writeConfig(t *testing.T, extraService, extraEndpoint string) string {
	t.Helper()
	cfg := `{
  "version": 3,
  "port": 18999,
  "extra_config": {` + extraService + `},
  "endpoints": [{
    "endpoint": "/live",
    "method": "GET",
    "extra_config": {` + extraEndpoint + `},
    "backend": [{"host": ["http://127.0.0.1:8081"], "url_pattern": "/x"}]
  }]
}`
	path := filepath.Join(t.TempDir(), "inkgate.json")
	if err := os.WriteFile(path, []byte(cfg), 0o600); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestCheck_acceptsAValidConfig(t *testing.T) {
	cfg := writeConfig(t, `"router": {"return_error_msg": true}`, `"router/stream": {"mode": "event", "keep_alive": "15s"}`)
	if out, code := inkgate(t, "check", "-c", cfg); code != 0 || !strings.Contains(out, "Syntax OK") {
		t.Fatalf("exit %d:\n%s", code, out)
	}
}

// A misspelled namespace used to turn its feature off without a word.
func TestCheckAndRun_refuseAMisspelledNamespace(t *testing.T) {
	cfg := writeConfig(t, ``, `"qos/ratelimt/router": {"max_rate": 10}`)
	for _, cmd := range []string{"check", "run"} {
		out, code := inkgate(t, cmd, "-c", cfg)
		if code != 1 {
			t.Errorf("%s: exit %d, want 1:\n%s", cmd, code, out)
		}
		for _, want := range []string{"/live", "qos/ratelimt/router", "qos/ratelimit/router"} { // where, what, did you mean
			if !strings.Contains(out, want) {
				t.Errorf("%s: the message must contain %q:\n%s", cmd, want, out)
			}
		}
	}
}

func TestCheckAndRun_refuseABadStreamSection(t *testing.T) {
	for name, section := range map[string]string{
		"misspelled option": `"router/stream": {"mode": "socket", "max_conections": 5}`,
		"not an object":     `"router/stream": true`,
	} {
		cfg := writeConfig(t, ``, section)
		for _, cmd := range []string{"check", "run"} {
			out, code := inkgate(t, cmd, "-c", cfg)
			if code != 1 || !strings.Contains(out, "/live") || !strings.Contains(out, "router/stream") {
				t.Errorf("%s, %s: exit %d, want 1 and a message naming the endpoint and router/stream:\n%s", name, cmd, code, out)
			}
		}
	}
}

func TestCheck_allowUnknownNamespacesIsTheOptOut(t *testing.T) {
	cfg := writeConfig(t, `"github_com/devopsfaith/krakend-cors": {}`, ``)
	if _, code := inkgate(t, "check", "-c", cfg); code != 1 {
		t.Fatalf("a KrakenD namespace must be refused by default, exit %d", code)
	}
	if out, code := inkgate(t, "check", "-c", cfg, "--allow-unknown-namespaces"); code != 0 {
		t.Fatalf("with --allow-unknown-namespaces: exit %d\n%s", code, out)
	}
}

// `check -n` lints against the schema embedded in the binary. It was `{}`, which accepted anything.
func TestCheckNoNetworkLint_usesTheEmbeddedSchema(t *testing.T) {
	good := writeConfig(t, `"router": {"return_error_msg": true}`, `"router/stream": {"mode": "event"}`)
	if out, code := inkgate(t, "check", "-n", "-c", good); code != 0 {
		t.Fatalf("a valid config: exit %d\n%s", code, out)
	}

	// an option of a namespace, and a key of the service: the Go validation does not see either
	for name, extra := range map[string][2]string{
		"an option of a namespace": {``, `"qos/ratelimit/router": {"max_rte": 10}`},
		"a misplaced namespace":    {`"router/stream": {"mode": "event"}`, ``},
	} {
		cfg := writeConfig(t, extra[0], extra[1])
		if out, code := inkgate(t, "check", "-n", "-c", cfg); code != 1 {
			t.Errorf("%s: exit %d, want 1\n%s", name, code, out)
		}
	}

	typo := strings.Replace(mustRead(t, good), `"port"`, `"prt"`, 1)
	path := filepath.Join(t.TempDir(), "typo.json")
	if err := os.WriteFile(path, []byte(typo), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, code := inkgate(t, "check", "-n", "-c", path); code != 1 {
		t.Errorf("a misspelled service key: exit %d, want 1\n%s", code, out)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return string(b)
}
