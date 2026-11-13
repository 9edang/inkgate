package boot

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/9edang/inkgate/context"
	"github.com/9edang/inkgate/internal/env"

	cmd "github.com/9edang/inkgate/engine/cli"
	"github.com/9edang/inkgate/engine/config"
	flexibleconfig "github.com/9edang/inkgate/engine/config/flexibleconfig"
	koanf "github.com/9edang/inkgate/engine/config/koanf"
	"github.com/9edang/inkgate/schema"
)

// Main is the whole program of a router variant: it reads the command line,
// parses the config and calls Run. cmd/inkgate only passes entry.Router to it.
func Main(r Router) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		select {
		case sig := <-sigs:
			log.Println("Signal intercepted:", sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	RegisterEncoders()
	if r.RegisterEncoders != nil {
		r.RegisterEncoders()
	}

	for key, alias := range aliases {
		config.ExtraConfigAlias[alias] = key
	}

	cmd.KnownNamespaces = KnownNamespaces()
	cmd.CustomValidationFunc = validateEndpoints

	var cfg config.Parser
	cfg = koanf.New()
	if env.App().FCEnable != "" {
		cfg = flexibleconfig.NewTemplateParser(flexibleconfig.Config{
			Parser:    cfg,
			Partials:  env.App().FCPartials,
			Settings:  env.App().FCSettings,
			Path:      env.App().FCOut,
			Templates: env.App().FCTemplates,
		})
	}

	commandsToLoad := []cmd.Command{
		cmd.RunCommand,
		cmd.NewCheckCmd(schema.JSON),
		cmd.PluginCommand,
		cmd.VersionCommand,
		cmd.AuditCommand,
		NewTestPluginCmd(),
	}

	cmd.RunRouterFunc = r.TestRoutes
	cmd.DefaultRoot = cmd.NewRoot(cmd.RootCommand, commandsToLoad...)
	cmd.DefaultRoot.Cmd.Use = "inkgate"
	cmd.DefaultRoot.Cmd.CompletionOptions.DisableDefaultCmd = true

	cmd.Execute(cfg, NewExecutor(ctx, r))
}

var aliases = map[string]string{
	"inkgate/engine/transport/http/server/plugin": "plugin/http-server",
	"inkgate/engine/transport/http/client/plugin": "plugin/http-client",
	"inkgate/engine/proxy/plugin":                 "plugin/req-resp-modifier",
	"inkgate/engine/proxy":                        "proxy",
	"inkgate/engine/router":                       "router",
	"inkgate/engine/router/stream":                "router/stream",
	"inkgate/context":                             "context",

	"inkgate/feature/traffic/httpcache":                "qos/http-cache",
	"inkgate/feature/traffic/circuitbreaker/gobreaker": "qos/circuit-breaker",

	"inkgate/feature/auth/oauth2client":   "auth/client-credentials",
	"inkgate/feature/auth/jose/validator": "auth/validator",
	"inkgate/feature/auth/jose/signer":    "auth/signer",
	"inkgate/feature/auth/bloomfilter":    "auth/revoker",

	"inkgate/feature/security/botdetector": "security/bot-detector",
	"inkgate/feature/security/httpsecure":  "security/http",
	"inkgate/feature/security/cors":        "security/cors",

	"inkgate/feature/validation/cel":        "validation/cel",
	"inkgate/feature/validation/jsonschema": "validation/json-schema",

	"inkgate/feature/connector/amqp/agent": "async/amqp",

	"inkgate/feature/connector/amqp/consume":       "backend/amqp/consumer",
	"inkgate/feature/connector/amqp/produce":       "backend/amqp/producer",
	"inkgate/feature/connector/lambda":             "backend/lambda",
	"inkgate/feature/connector/pubsub/publisher":   "backend/pubsub/publisher",
	"inkgate/feature/connector/pubsub/subscriber":  "backend/pubsub/subscriber",
	"inkgate/engine/transport/http/client/graphql": "backend/graphql",
	"inkgate/engine/transport/http/client":         "backend/http",

	"inkgate/feature/telemetry/gelf":      "telemetry/gelf",
	"inkgate/feature/telemetry/gologging": "telemetry/logging",
	"inkgate/feature/telemetry/logstash":  "telemetry/logstash",
	"inkgate/feature/telemetry/metrics":   "telemetry/metrics",
	"inkgate/feature/telemetry/influx":    "telemetry/influx",

	"inkgate/feature/transform/lua/router":        "modifier/lua-endpoint",
	"inkgate/feature/transform/lua/proxy":         "modifier/lua-proxy",
	"inkgate/feature/transform/lua/proxy/backend": "modifier/lua-backend",
	"inkgate/feature/transform/martian":           "modifier/martian",
}
