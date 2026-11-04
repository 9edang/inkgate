package config

import (
	"bytes"
	"errors"
	"github.com/9edang/inkgate/internal/json"

	coreconfig "github.com/9edang/inkgate/engine/config"
)

const (
	// Namespace is the key under the inkgate's "extra_config" root
	// section, for a valid config. See [config] documentation for
	// details.
	Namespace = "telemetry/opentelemetry"
)

// ErrNoConfig is used to signal no config was found
var ErrNoConfig = errors.New("no config found for opentelemetry")

// FromGateway extracts the configuration from the inkgate's ServiceConfig
// "extra_config" field.
//
// In case no "Layers" config is provided, a set of defaults with
// everything enabled will be used.
func FromGateway(srvCfg coreconfig.ServiceConfig) (*ConfigData, error) {
	cfg, err := GatewayExtraCfg(srvCfg.ExtraConfig)
	if err != nil {
		return nil, err
	}

	if cfg.ServiceName == "" {
		if srvCfg.Name != "" {
			cfg.ServiceName = srvCfg.Name
		} else {
			cfg.ServiceName = "Inkgate"
		}
	}

	cfg.UnsetFieldsToDefaults()
	return cfg, nil
}

// GatewayExtraCfg extracts the extra config field for the namespace if
// provided
func GatewayExtraCfg(extraCfg coreconfig.ExtraConfig) (*ConfigData, error) {
	tmp, ok := extraCfg[Namespace]
	if !ok {
		return nil, ErrNoConfig
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(tmp); err != nil {
		return nil, err
	}

	cfg := new(ConfigData)
	if err := json.NewDecoder(buf).Decode(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

func GatewayLayerExtraCfg(extraCfg coreconfig.ExtraConfig) (*LayersOpts, error) {
	tmp, ok := extraCfg[Namespace]
	if !ok {
		return nil, ErrNoConfig
	}

	buf := new(bytes.Buffer)
	if err := json.NewEncoder(buf).Encode(tmp); err != nil {
		return nil, err
	}

	cfg := new(LayersOpts)
	if err := json.NewDecoder(buf).Decode(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
