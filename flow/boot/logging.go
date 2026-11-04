package boot

import (
	"io"
	"os"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/logging"
	gelf "github.com/9edang/inkgate/feature/telemetry/gelf"
	gologging "github.com/9edang/inkgate/feature/telemetry/gologging"
	logstash "github.com/9edang/inkgate/feature/telemetry/logstash"
)

// newLogger builds the logger from the service config. The first configured
// backend wins: logstash, then gologging, then a plain stdout logger.
// The returned writer is the GELF writer (nil when GELF is not configured).
func newLogger(cfg config.ServiceConfig) (logging.Logger, io.Writer, error) {
	var writers []io.Writer
	gelfWriter, gelfErr := gelf.NewWriter(cfg.ExtraConfig)
	if gelfErr == nil {
		writers = append(writers, gelfWriterWrapper{gelfWriter})
		gologging.SetFormatterSelector(func(w io.Writer) string {
			switch w.(type) {
			case gelfWriterWrapper:
				return "%{message}"
			default:
				return gologging.DefaultPattern
			}
		})
	} else {
		gelfWriter = nil
	}

	logger, gologgingErr := logstash.NewLogger(cfg.ExtraConfig)
	if gologgingErr != nil {
		logger, gologgingErr = gologging.NewLogger(cfg.ExtraConfig, writers...)

		if gologgingErr != nil {
			var err error
			logger, err = logging.NewLogger("DEBUG", os.Stdout, "INKGATE")
			if err != nil {
				return logger, gelfWriter, err
			}
			if gologgingErr != gologging.ErrWrongConfig {
				logger.Error("[SERVICE: Logging] Unable to create the logger:", gologgingErr.Error())
			}
		}
	}

	if gelfErr != nil && gelfErr != gelf.ErrWrongConfig {
		logger.Error("[SERVICE: Logging][GELF] Unable to create the writer:", gelfErr.Error())
	}
	if gologgingErr == nil {
		logger.Debug("[SERVICE: telemetry/logging] Improved logging started.")
	}

	return logger, gelfWriter, nil
}

type gelfWriterWrapper struct {
	io.Writer
}
