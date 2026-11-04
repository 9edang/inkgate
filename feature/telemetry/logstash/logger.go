// Package logstash provides a logstash formatter for the inkgate-gologging pkg
package logstash

import (
	"errors"
	"fmt"
	"github.com/9edang/inkgate/internal/json"
	"io"
	"os"
	"time"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	"github.com/9edang/inkgate/engine/proxy"
	gologging "github.com/9edang/inkgate/feature/telemetry/gologging"
)

const Namespace = "inkgate/feature/telemetry/logstash"

var (
	ErrNothingToLog = errors.New("nothing to log")
	ErrWrongConfig  = fmt.Errorf("getting the extra config for the inkgate-logstash module")
	hostname        = "localhost"
	loggingPattern  = "%{message}"
)

func init() {
	name, err := os.Hostname()
	if err == nil {
		hostname = name
	}
}

// NewLogger returns a inkgate logger wrapping a gologging logger
func NewLogger(cfg config.ExtraConfig, ws ...io.Writer) (logging.Logger, error) {
	_, ok := cfg[Namespace]
	if !ok {
		return nil, ErrWrongConfig
	}
	serviceName := "INKGATE"
	gologging.DefaultPattern = loggingPattern
	if tmp, ok := cfg[gologging.Namespace]; ok {
		if section, ok := tmp.(map[string]any); ok {
			if tmp, ok = section["prefix"]; ok {
				if v, ok := tmp.(string); ok {
					serviceName = v
				}
				delete(section, "prefix")
			}
		}
	}

	loggr, err := gologging.NewLogger(cfg, ws...)
	if err != nil {
		return nil, err
	}

	return &Logger{loggr, serviceName}, nil
}

// Logger is a wrapper over a inkgate/engine/logging logger
type Logger struct {
	logger      logging.Logger
	serviceName string
}

var now = time.Now

func (l *Logger) format(logLevel LogLevel, v ...any) ([]byte, error) {
	if len(v) == 0 {
		return []byte{}, ErrNothingToLog
	}

	msg, ok := v[0].(string)
	if !ok {
		msg = fmt.Sprintf("%+v", v[0])
	}
	record := map[string]any{}
	if len(v) > 1 {
		for _, ctx := range v[1:] {
			switch value := ctx.(type) {
			case map[string]any:
				for k, item := range value {
					record[k] = item
				}
			case int:
				msg = fmt.Sprintf("%s %d", msg, value)
			case bool:
				msg = fmt.Sprintf("%s %t", msg, value)
			case float64:
				msg = fmt.Sprintf("%s %f", msg, value)
			case string:
				msg += " " + value
			case *proxy.Request:
				record["proxy.Request"] = value
			case *proxy.Response:
				record["proxy.Response"] = value
			default:
				record[fmt.Sprintf("%T", ctx)] = ctx
			}
		}
	}

	record["@version"] = 1
	record["@timestamp"] = now().Format(ISO_8601)
	record["module"] = l.serviceName
	record["host"] = hostname
	record["message"] = msg
	record["level"] = logLevel

	return json.Marshal(record)
}

// Debug implements the logger interface
func (l *Logger) Debug(v ...any) {
	data, err := l.format(LEVEL_DEBUG, v...)
	if err != nil {
		return
	}
	l.logger.Debug(string(data))
}

// Info implements the logger interface
func (l *Logger) Info(v ...any) {
	data, err := l.format(LEVEL_INFO, v...)
	if err != nil {
		return
	}
	l.logger.Info(string(data))
}

// Warning implements the logger interface
func (l *Logger) Warning(v ...any) {
	data, err := l.format(LEVEL_WARNING, v...)
	if err != nil {
		return
	}
	l.logger.Warning(string(data))
}

// Error implements the logger interface
func (l *Logger) Error(v ...any) {
	data, err := l.format(LEVEL_ERROR, v...)
	if err != nil {
		return
	}
	l.logger.Error(string(data))
}

// Critical implements the logger interface
func (l *Logger) Critical(v ...any) {
	data, err := l.format(LEVEL_CRITICAL, v...)
	if err != nil {
		return
	}
	l.logger.Critical(string(data))
}

// Fatal implements the logger interface
func (l *Logger) Fatal(v ...any) {
	data, err := l.format(LEVEL_CRITICAL, v...)
	if err != nil {
		return
	}
	l.logger.Fatal(string(data))
}

type LogLevel string

const (
	LEVEL_DEBUG    = "DEBUG"
	LEVEL_INFO     = "INFO"
	LEVEL_WARNING  = "WARNING"
	LEVEL_ERROR    = "ERROR"
	LEVEL_CRITICAL = "CRITICAL"

	ISO_8601 = "2006-01-02T15:04:05.000000-07:00"
)
