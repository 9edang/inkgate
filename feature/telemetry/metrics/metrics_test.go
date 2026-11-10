package metrics

import (
	"bytes"
	"math"
	"strings"
	"testing"
	"time"

	"github.com/9edang/inkgate/context"

	"github.com/rcrowley/go-metrics"

	"github.com/9edang/inkgate/engine/logging"
)

func TestConfigGetter(t *testing.T) {
	sampleCfg := map[string]any{
		Namespace: map[string]any{
			"proxy_disabled":  true,
			"router_disabled": true,
			"collection_time": "100ms",
			"listen_address":  "192.168.1.1:8888",
		},
	}
	testCfg := ConfigGetter(sampleCfg).(*Config)
	if testCfg.BackendDisabled {
		t.Error("Backend should be enabled.")
	}
	if !testCfg.ProxyDisabled {
		t.Error("Proxy should be disabled.")
	}
	if !testCfg.RouterDisabled {
		t.Error("Router should be disabled.")
	}
	if testCfg.CollectionTime != 100*time.Millisecond {
		t.Errorf("Unexpected collection time: %v", testCfg.CollectionTime)
	}
	if testCfg.ListenAddr != "192.168.1.1:8888" {
		t.Errorf("Unexpected addr: %s", testCfg.ListenAddr)
	}
}

func TestDefaultConfiguration(t *testing.T) {
	errorCfg := map[string]any{
		Namespace: map[string]any{
			"proxy_disabled": "bad_value",
		},
	}

	testCfg := ConfigGetter(errorCfg).(*Config)
	if testCfg.BackendDisabled {
		t.Error("The backend should be enabled by default.")
	}
	if testCfg.ProxyDisabled {
		t.Error("The proxy should be enabled by default.")
	}
	if testCfg.RouterDisabled {
		t.Error("The router should be enabled by default.")
	}
	if testCfg.CollectionTime != time.Minute {
		t.Errorf("Collection time should be 1 minute not %v", testCfg.CollectionTime)
	}
}

func TestNoConfiguration(t *testing.T) {
	noCfg := map[string]any{}
	testCfg := ConfigGetter(noCfg)
	if nil != testCfg {
		t.Error("The config should be nil (disabled middleware).")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	buf := bytes.NewBuffer(make([]byte, 1024))
	l, _ := logging.NewLogger("DEBUG", buf, "")
	m := New(ctx, nil, l)
	stats1 := m.Snapshot()
	time.Sleep(100 * time.Millisecond)
	stats2 := m.Snapshot()

	if stats1.Time > stats2.Time {
		t.Error("the later stat must have a higher timestamp")
		return
	}
	// sleep some time so the producer is able to collect some logs
	time.Sleep(200 * time.Millisecond)
	lines := len(strings.Split(buf.String(), "\n"))
	if lines != 1 {
		t.Error("unexpected log size. got:", lines)
	}
}

func TestBadConfiguration(t *testing.T) {
	invalidExtra := map[string]any{Namespace: true}
	testCfg := ConfigGetter(invalidExtra)
	if nil != testCfg {
		t.Error("The config should be nil (invalid ExtraConfig).")
	}

	badCfg := map[string]any{
		Namespace: map[string]any{"test": math.Inf},
	}
	if testCfg := ConfigGetter(badCfg); testCfg != nil {
		tmp := testCfg.(*Config)
		if tmp.BackendDisabled {
			t.Error("The backend should be enabled by default.")
		}
		if tmp.ProxyDisabled {
			t.Error("The proxy should be enabled by default.")
		}
		if tmp.RouterDisabled {
			t.Error("The router should be enabled by default.")
		}
		if tmp.CollectionTime != time.Minute {
			t.Errorf("Collection time should be 1 minute not %v", tmp.CollectionTime)
		}
	}
}

func TestMetrics(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	l, _ := logging.NewLogger("DEBUG", new(bytes.Buffer), "")
	cfg := map[string]any{Namespace: map[string]any{"collection_time": "100ms"}}
	m := New(ctx, cfg, l)
	stats1 := m.Snapshot()
	time.Sleep(100 * time.Millisecond)
	stats2 := m.Snapshot()

	if stats1.Time > stats2.Time {
		t.Error("the later stat must have a higher timestamp")
		return
	}
}

func TestMetrics_process(t *testing.T) {
	p := metrics.NewRegistry()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	called := false
	m := Metrics{Registry: &p, Router: NewRouterMetrics(&p), latestSnapshot: NewStats()}
	m.processMetrics(ctx, time.Millisecond, customLogger{&called})
	time.Sleep(50 * time.Millisecond)
	totalMetrics := 0
	tm := &totalMetrics
	expected := map[string]bool{
		"router.disconnected-total": true,
		"router.disconnected":       true,
		"router.disconnected-gauge": true,
		"router.connected-total":    true,
		"router.connected":          true,
		"router.connected-gauge":    true,
	}
	p.Each(func(name string, _ any) {
		_, isRouterMetric := expected[name]
		if !strings.HasPrefix(name, "service.") && !isRouterMetric {
			t.Errorf("Unexpected metric: %s", name)
		}
		delete(expected, name)
		*tm = *tm + 1
	})
	// The runtime stats are not counted: go-metrics registers them into the first registry of the process only
	// (registerRuntimeMetricsOnce), so their number depends on which test ran first.
	for name := range expected {
		t.Errorf("missing router metric %s", name)
	}
	if totalMetrics == 0 {
		t.Error("no metrics registered")
	}
}

type customLogger struct {
	called *bool
}

func (l customLogger) Printf(_ string, _ ...any) {
	*(l.called) = true
}
