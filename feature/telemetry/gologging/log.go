// Package gologging is the leveled logger of the telemetry/logging namespace. It is built on log/slog: a Handler renders
// every record with the configured pattern (the %{time}, %{level}, %{message}... syntax this logger always had) and
// writes it to each writer, and Logger puts that behind the engine's logging.Logger interface.
package gologging

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"log/syslog"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
)

// Namespace is the key to look for extra configuration details
const Namespace = "inkgate/feature/telemetry/gologging"

var (
	// ErrWrongConfig is the error returned when there is no config under the namespace.
	ErrWrongConfig = fmt.Errorf("getting the extra config for the inkgate-gologging module")
	// ErrInvalidLogLevel is the error returned for a level that is not one of DEBUG, INFO, NOTICE, WARNING, ERROR, CRITICAL.
	ErrInvalidLogLevel = errors.New("logger: invalid log level")
	// DefaultPattern is the pattern to use for rendering the logs
	LogstashPattern          = `{"@timestamp":"%{time:2006-01-02T15:04:05.000+00:00}", "@version": 1, "level": "%{level}", "message": "%{message}", "module": "%{module}"}`
	DefaultPattern           = ` %{time:2006/01/02 - 15:04:05.000} %{color}▶ %{level}%{color:reset} %{message}`
	ActivePattern            = DefaultPattern
	defaultFormatterSelector = func(io.Writer) string { return ActivePattern }
	defaultSyslogFacility    = syslog.LOG_LOCAL3
	defaultSyslogSeverity    = syslog.LOG_CRIT
)

// levelCritical is above slog's Error: slog has no level for it.
const levelCritical = slog.LevelError + 4

// SetFormatterSelector sets the ddefaultFormatterSelector function
func SetFormatterSelector(f func(io.Writer) string) {
	defaultFormatterSelector = f
}

// NewLogger returns a inkgate logger on top of log/slog.
func NewLogger(cfg config.ExtraConfig, ws ...io.Writer) (logging.Logger, error) {
	logConfig, ok := ConfigGetter(cfg).(Config)
	if !ok {
		return nil, ErrWrongConfig
	}
	const module = "INKGATE"

	if logConfig.StdOut {
		ws = append(ws, os.Stdout)
	}

	if logConfig.Syslog {
		var err error
		var w *syslog.Writer
		w, err = syslog.New(logConfig.SyslogSeverity|logConfig.SysLogFacility, logConfig.Prefix)
		if err != nil {
			return nil, err
		}
		ws = append(ws, syslogIoWriterWrapper{w})
	}

	if logConfig.Format == "logstash" {
		ActivePattern = LogstashPattern
		logConfig.Prefix = ""
	}

	if logConfig.Format == "custom" {
		ActivePattern = logConfig.CustomFormat
		logConfig.Prefix = ""
	}

	level, err := parseLevel(logConfig.Level)
	if err != nil {
		return nil, err
	}

	h := &handler{module: module, level: level}
	for _, w := range ws {
		var pattern string
		var prefix string
		switch w.(type) {
		case syslogIoWriterWrapper:
			pattern = "%{level} > %{message}"
		default:
			prefix = logConfig.Prefix
			pattern = defaultFormatterSelector(w)
		}
		parts, err := parsePattern(pattern)
		if err != nil {
			return nil, err
		}
		h.sinks = append(h.sinks, &sink{w: w, prefix: prefix, parts: parts})
	}

	return Logger{slog.New(h)}, nil
}

// parseLevel reads a level name, case insensitive. NOTICE is kept for the configs that have it: nothing logs at it, so
// it shows the same as WARNING.
func parseLevel(name string) (slog.Level, error) {
	switch strings.ToUpper(name) {
	case "DEBUG":
		return slog.LevelDebug, nil
	case "INFO":
		return slog.LevelInfo, nil
	case "NOTICE":
		return slog.LevelInfo + 2, nil
	case "WARNING":
		return slog.LevelWarn, nil
	case "ERROR":
		return slog.LevelError, nil
	case "CRITICAL":
		return levelCritical, nil
	}

	return 0, ErrInvalidLogLevel
}

func levelName(l slog.Level) string {
	switch {
	case l >= levelCritical:
		return "CRITICAL"
	case l >= slog.LevelError:
		return "ERROR"
	case l >= slog.LevelWarn:
		return "WARNING"
	case l >= slog.LevelInfo:
		return "INFO"
	}

	return "DEBUG"
}

// ANSI colors per level, as the %{color} variable writes them. INFO has none.
func levelColor(l slog.Level, bold bool) string {
	var code string
	switch {
	case l >= levelCritical:
		code = "35"
	case l >= slog.LevelError:
		code = "31"
	case l >= slog.LevelWarn:
		code = "33"
	case l >= slog.LevelInfo:
		return ""
	default:
		code = "36"
	}
	if bold {
		return "\033[" + code + ";1m"
	}

	return "\033[" + code + "m"
}

// A pattern is static text and variables, written %{name} or %{name:layout}. Variables: time (the layout is a Go time
// layout), level, module, message, id, pid, program (the layout is a fmt verb, e.g. %{level:-8s}), and color
// (%{color}, %{color:bold}, %{color:reset}).
var patternVar = regexp.MustCompile(`%{([a-z]+)(?::(.*?[^\\]))?}`)

var (
	pid     = os.Getpid()
	program = filepath.Base(os.Args[0])
)

type part struct {
	verb   string // "" for static text
	layout string
}

func parsePattern(pattern string) ([]part, error) {
	matches := patternVar.FindAllStringSubmatchIndex(pattern, -1)
	if matches == nil {
		return nil, errors.New("logger: invalid log format: " + pattern)
	}
	var parts []part
	prev := 0
	for _, m := range matches {
		if m[0] > prev {
			parts = append(parts, part{layout: pattern[prev:m[0]]})
		}
		name := pattern[m[2]:m[3]]
		layout, hasLayout := "", m[4] != -1
		if hasLayout {
			layout = pattern[m[4]:m[5]]
		}
		switch name {
		case "time":
			if !hasLayout {
				layout = "2006-01-02T15:04:05.999Z07:00"
			}
		case "color":
		case "level", "module", "message", "program":
			if !hasLayout {
				layout = "s"
			}
			layout = "%" + layout
		case "id", "pid":
			if !hasLayout {
				layout = "d"
			}
			layout = "%" + layout
		default:
			// file, line, function and call path variables are not offered: slog records the call site of the wrapper,
			// which would print the same place for every line
			return nil, errors.New("logger: unsupported variable in the log format: " + name)
		}
		parts = append(parts, part{verb: name, layout: layout})
		prev = m[1]
	}
	if prev < len(pattern) {
		parts = append(parts, part{layout: pattern[prev:]})
	}

	return parts, nil
}

// sink is one writer with its prefix and pattern.
type sink struct {
	mu     sync.Mutex
	w      io.Writer
	prefix string
	parts  []part
}

func (s *sink) write(r slog.Record, module string, id uint64) {
	var b strings.Builder
	b.WriteString(s.prefix)
	for _, p := range s.parts {
		switch p.verb {
		case "":
			b.WriteString(p.layout)
		case "time":
			b.WriteString(r.Time.Format(p.layout))
		case "color":
			switch p.layout {
			case "reset":
				b.WriteString("\033[0m")
			case "bold":
				b.WriteString(levelColor(r.Level, true))
			default:
				b.WriteString(levelColor(r.Level, false))
			}
		case "level":
			fmt.Fprintf(&b, p.layout, levelName(r.Level))
		case "module":
			fmt.Fprintf(&b, p.layout, module)
		case "message":
			fmt.Fprintf(&b, p.layout, r.Message)
		case "id":
			fmt.Fprintf(&b, p.layout, id)
		case "pid":
			fmt.Fprintf(&b, p.layout, pid)
		case "program":
			fmt.Fprintf(&b, p.layout, program)
		}
	}
	b.WriteByte('\n')

	s.mu.Lock()
	defer s.mu.Unlock()
	// #nosec G104 -- a log line that cannot be written has nowhere to be reported
	_, _ = io.WriteString(s.w, b.String())
}

// handler is the slog.Handler of the logger: it renders a record once per writer, with that writer's pattern.
type handler struct {
	module string
	level  slog.Level
	sinks  []*sink
	seq    atomic.Uint64
}

func (h *handler) Enabled(_ context.Context, l slog.Level) bool { return l >= h.level }

func (h *handler) Handle(_ context.Context, r slog.Record) error {
	id := h.seq.Add(1)
	for _, s := range h.sinks {
		s.write(r, h.module, id)
	}

	return nil
}

// The logger carries no attributes or groups: the interface it serves is messages only.
func (h *handler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *handler) WithGroup(string) slog.Handler      { return h }

// ConfigGetter implements the config.ConfigGetter interface
func ConfigGetter(e config.ExtraConfig) any {
	v, ok := e[Namespace]
	if !ok {
		return nil
	}
	tmp, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	cfg := Config{}
	if v, ok := tmp["stdout"]; ok {
		cfg.StdOut = v.(bool)
	}
	if v, ok := tmp["syslog"]; ok {
		cfg.Syslog = v.(bool)
	}

	cfg.SysLogFacility = defaultSyslogFacility
	if v, ok := tmp["syslog_facility"].(string); ok {
		cfg.SysLogFacility = parseSyslogFacility(v)
	}

	if v, ok := tmp["level"].(string); ok {
		cfg.Level = v
	}
	cfg.SyslogSeverity = parseSyslogSeverity(cfg.Level)

	if v, ok := tmp["prefix"].(string); ok {
		cfg.Prefix = v
	}
	if v, ok := tmp["format"].(string); ok {
		cfg.Format = v
	}
	if v, ok := tmp["custom_format"].(string); ok {
		cfg.CustomFormat = v
	}
	return cfg
}

// Config is the custom config struct containing the params for the logger
type Config struct {
	Level          string
	StdOut         bool
	Syslog         bool
	SysLogFacility syslog.Priority
	SyslogSeverity syslog.Priority
	Prefix         string
	Format         string
	CustomFormat   string
}

// Logger puts a slog.Logger behind the engine's logging.Logger interface.
type Logger struct {
	logger *slog.Logger
}

// Enabled implements logging.LevelChecker.
func (l Logger) Enabled(level int) bool {
	return l.logger.Enabled(context.Background(), slogLevel(level))
}

func slogLevel(level int) slog.Level {
	switch level {
	case logging.LEVEL_DEBUG:
		return slog.LevelDebug
	case logging.LEVEL_INFO:
		return slog.LevelInfo
	case logging.LEVEL_WARNING:
		return slog.LevelWarn
	case logging.LEVEL_ERROR:
		return slog.LevelError
	default:
		return levelCritical
	}
}

// log formats the arguments like fmt.Sprintln does (a space between all of them), as this logger always has.
func (l Logger) log(level slog.Level, v ...any) {
	ctx := context.Background()
	if !l.logger.Enabled(ctx, level) {
		return
	}
	msg := fmt.Sprintln(v...)
	l.logger.Log(ctx, level, msg[:len(msg)-1])
}

// Debug implements the logger interface.
func (l Logger) Debug(v ...any) { l.log(slog.LevelDebug, v...) }

// Info implements the logger interface
func (l Logger) Info(v ...any) { l.log(slog.LevelInfo, v...) }

// Warning implements the logger interface
func (l Logger) Warning(v ...any) { l.log(slog.LevelWarn, v...) }

// Error implements the logger interface
func (l Logger) Error(v ...any) { l.log(slog.LevelError, v...) }

// Critical implements the logger interface
func (l Logger) Critical(v ...any) { l.log(levelCritical, v...) }

// Fatal implements the logger interface: it logs at CRITICAL and exits with status 1.
func (l Logger) Fatal(v ...any) {
	l.log(levelCritical, v...)
	os.Exit(1)
}

type syslogIoWriterWrapper struct {
	io.Writer
}

func parseSyslogFacility(name string) syslog.Priority {
	switch strings.ToLower(name) {
	case "local0":
		return syslog.LOG_LOCAL0
	case "local1":
		return syslog.LOG_LOCAL1
	case "local2":
		return syslog.LOG_LOCAL2
	case "local3":
		return syslog.LOG_LOCAL3
	case "local4":
		return syslog.LOG_LOCAL4
	case "local5":
		return syslog.LOG_LOCAL5
	case "local6":
		return syslog.LOG_LOCAL6
	case "local7":
		return syslog.LOG_LOCAL7
	default:
		return defaultSyslogFacility
	}
}

func parseSyslogSeverity(level string) syslog.Priority {
	switch strings.ToLower(level) {
	case "fatal":
		return syslog.LOG_EMERG
	case "critical":
		return syslog.LOG_CRIT
	case "error":
		return syslog.LOG_ERR
	case "warning":
		return syslog.LOG_WARNING
	case "info":
		return syslog.LOG_INFO
	case "debug":
		return syslog.LOG_DEBUG
	default:
		return defaultSyslogSeverity
	}
}
