package koanf

import (
	"github.com/9edang/inkgate/internal/json"
	"strings"
	"testing"
)

func TestNew_ok(t *testing.T) {
	configContents := []struct {
		format string
		path   string
	}{
		{"json", "./fixtures/ok.json"},
		{"toml", "./fixtures/ok.toml"},
		{"yaml", "./fixtures/ok.yaml"},
		{"yml", "./fixtures/ok.yml"},
	}
	for _, configContent := range configContents {
		t.Run(configContent.format, func(t *testing.T) {
			serviceConfig, err := New().ParseWithoutInit(configContent.path)
			if err != nil {
				t.Error("Unexpected error. Got", err.Error())
				return
			}

			endpoint := serviceConfig.Endpoints[0]
			endpointExtraConfiguration := endpoint.ExtraConfig

			if endpointExtraConfiguration != nil {
				testExtraConfig(endpointExtraConfiguration, t)
			} else {
				t.Errorf("Extra config is not present in EndpointConfig: %#v", endpoint)
				return
			}

			backend := endpoint.Backend[0]
			backendExtraConfiguration := backend.ExtraConfig
			if backendExtraConfiguration != nil {
				testExtraConfig(backendExtraConfiguration, t)
			} else {
				t.Error("Extra config is not present in BackendConfig")
				return
			}
		})
	}
}

func TestNew_errorMessages(t *testing.T) {
	for _, configContent := range []struct {
		path   string
		expErr string
	}{
		{
			path:   "unexpected_end_of_json.json",
			expErr: "unexpected end of JSON input",
		},
		{
			path:   "invalid_character.json",
			expErr: "invalid character '>' looking for beginning of value",
		},
		{
			path:   "only_quotes.json",
			expErr: "invalid character",
		},
		{
			path:   "empty.json",
			expErr: "unexpected end of JSON input",
		},
		{
			path:   "array.json",
			expErr: "json: cannot unmarshal array into Go value of type map[string]interface {}",
		},
		{
			path:   "number.json",
			expErr: "json: cannot unmarshal number into Go value of type map[string]interface {}",
		},
		{
			path:   "space_and_number.json",
			expErr: "json: cannot unmarshal number into Go value of type map[string]interface {}",
		},
		{
			path:   "missing_comma.json",
			expErr: "invalid character '\"' after object key:value pair",
		},
	} {
		t.Run(configContent.path, func(t *testing.T) {
			_, err := New().ParseWithoutInit("./fixtures/" + configContent.path)
			if err == nil {
				t.Errorf("%s: Expecting error", configContent.path)
				return
			}
			if errMsg := err.Error(); !strings.Contains(errMsg, configContent.expErr) {
				t.Errorf("%s: Unexpected error. Got '%s' want '%s'", configContent.path, errMsg, configContent.expErr)
				return
			}
		})
	}
}

func testExtraConfig(extraConfig map[string]any, t *testing.T) {
	userVar := extraConfig["user"]
	if userVar != "test" {
		t.Error("User in extra config is not test")
	}
	parents := extraConfig["parents"].([]any)
	if parents[0] != "gomez" {
		t.Error("Parent 0 of user us not gomez")
	}
	if parents[1] != "morticia" {
		t.Error("Parent 1 of user us not morticia")
	}

	testExtraNestedConfigKey(extraConfig, t)
}

func testExtraNestedConfigKey(extraConfig map[string]any, t *testing.T) {
	namespace := "nested_data"
	v, ok := extraConfig[namespace]
	if !ok {
		return
	}

	type nestedConfig struct {
		Data struct {
			Status string `json:"status"`
		} `json:"data"`
	}

	jsonBytes, err := json.Marshal(v)
	if err != nil {
		t.Error("marshal nested config key error: ", err.Error())
		return
	}

	var cfg nestedConfig
	if err = json.Unmarshal(jsonBytes, &cfg); err != nil {
		t.Error("unmarshal nested config key error: ", err.Error())
		return
	}

	if cfg.Data.Status != "OK" {
		t.Errorf("nested config key parse error: %+v\n", cfg)
	}
}

func TestNew_unknownFile(t *testing.T) {
	_, err := New().ParseWithoutInit("/nowhere/in/the/fs.json")
	if err == nil || !strings.Contains(err.Error(), "no such file or directory") {
		t.Errorf("Error expected. Got '%s'", err)
	}
}

func TestNew_readingError(t *testing.T) {
	wrongConfigPath := "./fixtures/reading.json"
	expected := "'./fixtures/reading.json': invalid character 'h' looking for beginning of object key string"
	_, err := New().ParseWithoutInit(wrongConfigPath)
	if err == nil || err.Error() != expected {
		t.Errorf("Error expected. Got '%s'", err)
	}
}

func TestNew_initError(t *testing.T) {
	wrongConfigPath := "./fixtures/unmarshal.json"
	_, err := New().Parse(wrongConfigPath)
	if err == nil || err.Error() != "'./fixtures/unmarshal.json': unsupported version: 0 (want: 3)" {
		t.Error("Error expected. Got", err)
	}
}
