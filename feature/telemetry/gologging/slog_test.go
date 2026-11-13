package gologging

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/9edang/inkgate/engine/logging"
)

func newTestLogger(t *testing.T, level, format, custom string) (logging.Logger, *bytes.Buffer) {
	t.Helper()
	var buf bytes.Buffer
	ActivePattern = DefaultPattern
	SetFormatterSelector(func(io.Writer) string { return ActivePattern })
	l, err := NewLogger(newExtraConfig(level, format, custom), &buf)
	if err != nil {
		t.Fatal(err)
	}

	return l, &buf
}

// The default format: prefix, time, the level in its color, the message; arguments are joined with a space.
func TestLogger_defaultFormat(t *testing.T) {
	l, buf := newTestLogger(t, "DEBUG", "default", "")
	l.Error("[BACKEND: /x]", "failed:", errors.New("boom"), 42)
	l.Info("hello")
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("want 2 lines, got %q", lines)
	}
	errLine := regexp.MustCompile(`^pref \d{4}/\d\d/\d\d - \d\d:\d\d:\d\d\.\d{3} \x1b\[31m▶ ERROR\x1b\[0m \[BACKEND: /x\] failed: boom 42$`)
	if !errLine.MatchString(lines[0]) {
		t.Errorf("error line: %q", lines[0])
	}
	if !strings.HasSuffix(lines[1], " ▶ INFO\x1b[0m hello") || strings.Contains(lines[1], "\x1b[3") {
		t.Errorf("info has no color: %q", lines[1])
	}
}

// Only what the level allows is written; NOTICE, which nothing logs at, behaves as WARNING.
func TestLogger_levelThreshold(t *testing.T) {
	for level, want := range map[string][]string{
		"DEBUG":    {"d", "i", "w", "e", "c"},
		"info":     {"i", "w", "e", "c"},
		"NOTICE":   {"w", "e", "c"},
		"WARNING":  {"w", "e", "c"},
		"ERROR":    {"e", "c"},
		"CRITICAL": {"c"},
	} {
		l, buf := newTestLogger(t, level, "custom", "%{message}")
		l.Debug("d")
		l.Info("i")
		l.Warning("w")
		l.Error("e")
		l.Critical("c")
		got := strings.Fields(buf.String())
		if strings.Join(got, "") != strings.Join(want, "") {
			t.Errorf("%s: wrote %v, want %v", level, got, want)
		}
		for lvl, name := range map[int]string{logging.LEVEL_DEBUG: "d", logging.LEVEL_INFO: "i", logging.LEVEL_WARNING: "w", logging.LEVEL_ERROR: "e", logging.LEVEL_CRITICAL: "c"} {
			enabled := logging.Enabled(l, lvl)
			if enabled != strings.Contains(strings.Join(want, ""), name) {
				t.Errorf("%s: Enabled(%s) = %v", level, name, enabled)
			}
		}
	}
}

// Custom formats: layouts of the variables, id, module; variables that need a call site are refused at startup, not
// at the first log line.
func TestNewLogger_customFormats(t *testing.T) {
	l, buf := newTestLogger(t, "DEBUG", "custom", "%{level:-8s}|%{module}|%{id}|%{message}")
	l.Warning("a")
	l.Error("b")
	if got, want := buf.String(), "WARNING |INKGATE|1|a\nERROR   |INKGATE|2|b\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	for _, bad := range []string{"%{shortfile} %{message}", "%{callpath}", "%{nosuch}", "no variable at all", ""} {
		SetFormatterSelector(func(io.Writer) string { return ActivePattern })
		if _, err := NewLogger(newExtraConfig("DEBUG", "custom", bad), io.Discard); err == nil {
			t.Errorf("format %q must be refused", bad)
		}
	}
}

// Lines written at the same time are never mixed.
func TestLogger_concurrentLinesStayWhole(t *testing.T) {
	l, buf := newTestLogger(t, "DEBUG", "custom", "%{message}")
	var wg sync.WaitGroup
	for i := range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				l.Info(strings.Repeat(string(rune('a'+i)), 40))
			}
		}()
	}
	wg.Wait()
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 1000 {
		t.Fatalf("%d lines, want 1000", len(lines))
	}
	for _, line := range lines {
		if len(line) != 40 || strings.Trim(line, line[:1]) != "" {
			t.Fatalf("mixed line %q", line)
		}
	}
}

func TestLevelNames(t *testing.T) {
	for l, want := range map[slog.Level]string{slog.LevelDebug: "DEBUG", slog.LevelInfo: "INFO", slog.LevelWarn: "WARNING", slog.LevelError: "ERROR", levelCritical: "CRITICAL"} {
		if got := levelName(l); got != want {
			t.Errorf("%v: %s, want %s", l, got, want)
		}
	}
}
