package env

// App returns the process-level settings read from the environment.
func App() ConfigApp {
	return app
}

type ConfigApp struct {
	// SchemaURL is the online JSON schema used by `check --lint`.
	SchemaURL string `env:"INKGATE_SCHEMA_URL"`

	// FCEnable turns on flexible config (templated config) when non-empty.
	FCEnable    string `env:"FC_ENABLE"`
	FCSettings  string `env:"FC_SETTINGS"`
	FCPartials  string `env:"FC_PARTIALS"`
	FCTemplates string `env:"FC_TEMPLATES"`
	FCOut       string `env:"FC_OUT"`
}
