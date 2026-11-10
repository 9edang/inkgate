package logstash

import (
	"bytes"
	"math"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/9edang/inkgate/internal/json"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	gologging "github.com/9edang/inkgate/feature/telemetry/gologging"
)

func TestNewLogger(t *testing.T) {
	logger, err := NewLogger(config.ExtraConfig{})
	if err == nil {
		t.Error("expecting an error due empty config")
		return
	}

	cfg := config.ExtraConfig{
		gologging.Namespace: map[string]any{
			"level":  LEVEL_DEBUG,
			"prefix": "module_name",
			"stdout": true,
		},
		Namespace: nil,
	}
	logger, err = NewLogger(cfg)
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
		return
	}

	logger.Debug("mayday, mayday, mayday", map[string]any{"a": true, "b": false, "cost": math.Pi})
}

func TestLogger_nothingToLog(t *testing.T) {
	buff := new(bytes.Buffer)
	l, _ := logging.NewLogger(LEVEL_DEBUG, buff, "")
	logger := Logger{
		logger:      l,
		serviceName: "some",
	}

	logger.Debug()
	logger.Info()
	logger.Warning()
	logger.Error()
	logger.Critical()

	if content := buff.String(); content != "" {
		t.Errorf("unexpected log content: %s", content)
	}
}

func TestLogger(t *testing.T) {
	expectedModuleName := "module_name"
	expectedMsg := "mayday, mayday, mayday"
	buff := new(bytes.Buffer)
	l, _ := logging.NewLogger(LEVEL_DEBUG, buff, "")
	logger := Logger{
		logger:      l,
		serviceName: expectedModuleName,
	}

	location, _ := time.LoadLocation("")
	now = func() time.Time {
		return time.Unix(1526464967, 0).In(location)
	}
	defer func() { now = time.Now }()

	logger.Debug(expectedMsg, map[string]any{"a": true, "b": false, "cost": 42})
	logger.Info(expectedMsg, map[string]any{"a": true, "b": false, "cost": 42})
	logger.Warning(expectedMsg, map[string]any{"a": true, "b": false, "cost": 42})
	logger.Error(expectedMsg, map[string]any{"a": true, "b": false, "cost": 42})
	logger.Critical(expectedMsg, map[string]any{"a": true, "b": false, "cost": 42})

	lines := strings.Split(buff.String(), "\n")
	if len(lines) < 5 {
		t.Errorf("unexpected number of logged lines (%d) : %s", len(lines), buff.String())
		return
	}
	levels := []string{"DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"}
	for line, logLine := range lines[:5] {
		if logLine[20:] == levels[line]+`: [{"@timestamp":"2018-05-16T10:02:47.000000+00:00","@version":1,"a":true,"b":false,"cost":42,"host":"localhost","level":"`+levels[line]+`","message":"mayday, mayday, mayday","module":"module_name"}]]` {
			t.Errorf("The output doesn't contain the expected msg for the line %d: [%s]", line, logLine)
		}
	}
}

func TestLogger_format(t *testing.T) {
	expectedModuleName := "module_name"
	expectedMsg := "mayday, mayday, mayday"
	cfg := config.ExtraConfig{
		gologging.Namespace: map[string]any{
			"level":  LEVEL_DEBUG,
			"prefix": expectedModuleName,
			"stdout": true,
		},
		Namespace: nil,
	}
	logger, err := NewLogger(cfg)
	if err != nil {
		t.Errorf("unexpected error: %s", err.Error())
		return
	}

	location, _ := time.LoadLocation("")
	now = func() time.Time {
		return time.Unix(1526464967, 0).In(location)
	}
	defer func() { now = time.Now }()

	for i, logLevel := range []LogLevel{
		LEVEL_DEBUG,
		LEVEL_INFO,
		LEVEL_WARNING,
		LEVEL_ERROR,
		LEVEL_CRITICAL,
	} {
		data, err := logger.(*Logger).format(logLevel, expectedMsg, map[string]any{"a": true, "b": false, "cost": 42})
		if err != nil {
			t.Errorf("unexpected error runing test case #%d: %s", i, err.Error())
			continue
		}
		var content map[string]any
		if err := json.Unmarshal(data, &content); err != nil {
			t.Errorf("unexpected error unmarshaling the message #%d: %s", i, err.Error())
			continue
		}

		expectedHostname, err := os.Hostname()
		if err != nil {
			t.Errorf("failed to get hostname runing test case #%d: %s", i, err.Error())
			continue
		}

		expectedMessage := map[string]any{
			"a":          true,
			"b":          false,
			"cost":       42.0,
			"@version":   1.0,
			"@timestamp": "2018-05-16T10:02:47.000000+00:00",
			"module":     expectedModuleName,
			"host":       expectedHostname,
			"message":    expectedMsg,
			"level":      string(logLevel),
		}

		for k, v := range content {
			tmp, ok := expectedMessage[k]
			if !ok {
				t.Errorf("no value for key %s", k)
				continue
			}

			if !sameValue(tmp, v) {
				t.Errorf("unexpected value for key %s. Have: %v, Want: %v", k, v, tmp)
			}
		}
	}
}

func TestLogger_format_unexpectedMessageType(t *testing.T) {
	buff := new(bytes.Buffer)
	l, _ := logging.NewLogger(LEVEL_DEBUG, buff, "")
	logger := Logger{
		logger:      l,
		serviceName: "some",
	}

	location, _ := time.LoadLocation("")
	now = func() time.Time {
		return time.Unix(1526464967, 0).In(location)
	}
	defer func() { now = time.Now }()

	expectedHostname, err := os.Hostname()
	if err != nil {
		t.Errorf("failed to get hostname: %s", err.Error())
		return
	}

	for i, testCase := range []struct {
		Expected string
		Values   []any
	}{
		{
			Expected: `{"@timestamp":"2018-05-16T10:02:47.000000+00:00","@version":1,"host":"` + expectedHostname + `","level":"DEBUG","message":"42","module":"some"}`,
			Values:   []any{42},
		},
		{
			Expected: `{"@timestamp":"2018-05-16T10:02:47.000000+00:00","@version":1,"a":1,"host":"` + expectedHostname + `","level":"DEBUG","message":"42","module":"some"}`,
			Values:   []any{42, map[string]any{"a": 1}},
		},
		{
			Expected: `{"@timestamp":"2018-05-16T10:02:47.000000+00:00","@version":1,"host":"` + expectedHostname + `","level":"DEBUG","logstash.sample":{"A":1},"message":"42","module":"some"}`,
			Values:   []any{42, sample{A: 1}},
		},
		{
			Expected: `{"@timestamp":"2018-05-16T10:02:47.000000+00:00","@version":1,"host":"` + expectedHostname + `","level":"DEBUG","message":"hey there multi parts","module":"some"}`,
			Values:   []any{"hey", "there", "multi", "parts"},
		},
		{
			Expected: `{"@timestamp":"2018-05-16T10:02:47.000000+00:00","@version":1,"host":"` + expectedHostname + `","level":"DEBUG","message":"true 3 1.100000 basic types true","module":"some"}`,
			Values:   []any{true, 3, 1.1, "basic types", true},
		},
		{
			Expected: `{"@timestamp":"2018-05-16T10:02:47.000000+00:00","@version":1,"host":"` + expectedHostname + `","level":"DEBUG","logstash.sample":{"A":1},"message":"true 3 1.100000 basic types true","module":"some"}`,
			Values:   []any{true, 3, sample{A: 1}, 1.1, "basic types", true},
		},
	} {
		data, err := logger.format(LEVEL_DEBUG, testCase.Values...)
		if err != nil {
			t.Errorf("unexpected error (#%d): %s", i, err.Error())
			continue
		}

		if string(data) != testCase.Expected {
			t.Errorf("unexpected result (#%d). Have: %s, Want: %s", i, string(data), testCase.Expected)
		}
	}
}

type sample struct{ A int }

// sameValue compares decoded JSON values; numbers by value, since the decoder may give float64 or int64
// (internal/json decodes whole numbers as int64 by default, JSON_USE_INT64).
func sameValue(want, have any) bool {
	if w, ok := config.Float64(want); ok {
		h, ok := config.Float64(have)
		return ok && w == h
	}

	return reflect.DeepEqual(want, have)
}

// M2 2.4: a level the inner logger drops is not formatted into a JSON record at all.
func TestLoggerSkipsDisabledLevels(t *testing.T) {
	buf := new(bytes.Buffer)
	l, err := NewLogger(config.ExtraConfig{
		Namespace:           map[string]any{},
		gologging.Namespace: map[string]any{"level": "ERROR", "stdout": false},
	}, buf)
	if err != nil {
		t.Fatal(err)
	}
	if logging.Enabled(l, logging.LEVEL_DEBUG) || !logging.Enabled(l, logging.LEVEL_ERROR) {
		t.Error("Enabled must follow the inner logger's level (ERROR)")
	}
	l.Debug("dropped")
	if buf.Len() != 0 {
		t.Errorf("a DEBUG line was written at ERROR: %q", buf.String())
	}
	l.Error("kept")
	if !strings.Contains(buf.String(), "kept") {
		t.Errorf("the ERROR line is missing: %q", buf.String())
	}
}
