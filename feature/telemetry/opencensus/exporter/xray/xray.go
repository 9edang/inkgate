package xray

import (
	"errors"
	"github.com/9edang/inkgate/context"
	"time"

	ocAws "contrib.go.opencensus.io/exporter/aws"
	opencensus "github.com/9edang/inkgate/feature/telemetry/opencensus"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/xray"
)

func init() {
	opencensus.RegisterExporterFactories(func(ctx context.Context, cfg opencensus.Config) (any, error) {
		return Exporter(ctx, cfg)
	})
}

func Exporter(_ context.Context, cfg opencensus.Config) (*ocAws.Exporter, error) {
	if cfg.Exporters.Xray == nil {
		return nil, errors.New("xray exporter disabled")
	}
	if cfg.Exporters.Xray.Version == "" {
		cfg.Exporters.Xray.Version = "Inkgate-opencensus"
	}

	if !cfg.Exporters.Xray.UseEnv {
		mySession := setupAWSSession(cfg.Exporters.Xray.AccessKey, cfg.Exporters.Xray.SecretKey, cfg.Exporters.Xray.Region)
		return ocAws.NewExporter(
			ocAws.WithAPI(xray.New(mySession, aws.NewConfig().WithRegion(cfg.Exporters.Xray.Region))),
			ocAws.WithRegion(cfg.Exporters.Xray.Region),
			ocAws.WithInterval(time.Duration(cfg.ReportingPeriod)),
			ocAws.WithBufferSize(cfg.SampleRate),
			ocAws.WithVersion(cfg.Exporters.Xray.Version),
		)
	}

	return ocAws.NewExporter(
		ocAws.WithRegion(cfg.Exporters.Xray.Region),
		ocAws.WithInterval(time.Duration(cfg.ReportingPeriod)),
		ocAws.WithBufferSize(cfg.SampleRate),
		ocAws.WithVersion(cfg.Exporters.Xray.Version),
	)
}

func setupAWSSession(id, secret, region string) *session.Session {
	return session.Must(session.NewSession(&aws.Config{
		Credentials: credentials.NewStaticCredentials(id, secret, ""),
		Region:      new(region),
	}))
}
