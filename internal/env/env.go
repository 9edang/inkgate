package env

import (
	"fmt"
	"os"

	"github.com/caarlos0/env/v11"
)

var (
	json ConfigJSON
	app  ConfigApp
)

func init() {
	if err := env.ParseWithOptions(&json, env.Options{SetDefaultsForZeroValuesOnly: true}); err != nil {
		panic(fmt.Errorf("error load env json config: %w", err))
	}

	if err := env.Parse(&app); err != nil {
		panic(fmt.Errorf("error load env app config: %w", err))
	}
}

func Environ() []string {
	return os.Environ()
}
