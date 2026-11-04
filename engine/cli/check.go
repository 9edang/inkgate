package cmd

import (
	"fmt"
	"github.com/9edang/inkgate/internal/json"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/9edang/inkgate/engine/cli/dumper"
	"github.com/9edang/inkgate/internal/env"

	"github.com/9edang/inkgate/engine/config"

	"github.com/spf13/cobra"
)

// SchemaURL is the online JSON schema used by `check --lint`. Set it with INKGATE_SCHEMA_URL.
var SchemaURL = env.App().SchemaURL

func errorMsg(content string) string {
	if !IsTTY {
		return content
	}
	return dumper.ColorRed + content + dumper.ColorReset
}

type LastSourcer interface {
	LastSource() ([]byte, error)
}

func NewCheckCmd(rawSchema string) Command {
	rawEmbedSchema = rawSchema
	return CheckCommand
}

func checkFunc(cmd *cobra.Command, _ []string) { // skipcq: GO-R1005
	if cfgFile == "" {
		cmd.Println(errorMsg("Please, provide the path to the configuration file with --config or see all the options with --help"))
		os.Exit(1) // skipcq: RVV-A0003
		return
	}

	cmd.Printf("Parsing configuration file: %s\n", cfgFile)

	v, err := parser.Parse(cfgFile)
	if err != nil {
		cmd.Println(errorMsg("ERROR parsing the configuration file:") + fmt.Sprintf("\t%s\n", err.Error()))
		os.Exit(1) // skipcq: RVV-A0003
		return
	}

	if customErrs := CustomValidationFunc(v); len(customErrs) > 0 {
		eb := strings.Builder{}
		for _, err := range customErrs {
			eb.WriteString(fmt.Sprintf("\t%s\n", err.Error()))
		}

		cmd.Println(errorMsg("ERROR validating the configuration file:\n") + eb.String())
		os.Exit(1) // skipcq: RVV-A0003
		return
	}

	shouldLint := lintCurrentSchema || lintNoNetwork || (lintCustomSchemaPath != "")

	if shouldLint {
		var data []byte
		var err error
		if ls, ok := parser.(LastSourcer); ok {
			data, err = ls.LastSource()
		} else {
			data, err = os.ReadFile(cfgFile)
		}

		if err != nil {
			cmd.Println(errorMsg("ERROR loading the configuration content:") + fmt.Sprintf("\t%s\n", err.Error()))
			os.Exit(1) // skipcq: RVV-A0003
			return
		}

		var raw any
		if err := json.Unmarshal(data, &raw); err != nil {
			cmd.Println(errorMsg("ERROR converting configuration content to JSON:") + fmt.Sprintf("\t%s\n", err.Error()))
			os.Exit(1) // skipcq: RVV-A0003
			return
		}

		var sch *jsonschema.Schema
		var compilationErr error
		if lintNoNetwork {
			rawSchema, parseError := jsonschema.UnmarshalJSON(strings.NewReader(rawEmbedSchema))
			if parseError != nil {
				cmd.Println(errorMsg("ERROR parsing the embed schema:") + fmt.Sprintf("\t%s\n", parseError.Error()))
				os.Exit(1) // skipcq: RVV-A0003
				return
			}

			compiler := jsonschema.NewCompiler()
			compiler.AddResource("schema.json", rawSchema)

			sch, compilationErr = compiler.Compile("schema.json")
		} else {
			if lintCustomSchemaPath == "" {
				if SchemaURL == "" {
					cmd.Println(errorMsg("ERROR linting the configuration file:") + "\tno online schema configured: set INKGATE_SCHEMA_URL or use --lint-schema\n")
					os.Exit(1) // skipcq: RVV-A0003
					return
				}
				lintCustomSchemaPath = SchemaURL
			}

			httpLoader := SchemaHttpLoader(http.Client{
				Timeout: 10 * time.Second,
			})

			loader := jsonschema.SchemeURLLoader{
				"file":  jsonschema.FileLoader{},
				"http":  &httpLoader,
				"https": &httpLoader,
			}
			compiler := jsonschema.NewCompiler()
			compiler.UseLoader(loader)

			sch, compilationErr = compiler.Compile(lintCustomSchemaPath)
		}

		if compilationErr != nil {
			cmd.Println(errorMsg("ERROR compiling the schema:") + fmt.Sprintf("\t%s\n", compilationErr.Error()))
			os.Exit(1) // skipcq: RVV-A0003
			return
		}

		if err = sch.Validate(raw); err != nil {
			cmd.Println(errorMsg("ERROR linting the configuration file:") + fmt.Sprintf("\t%s\n", err.Error()))
			os.Exit(1) // skipcq: RVV-A0003
			return
		}
	}

	if checkDebug > 0 {
		cc := dumper.NewWithColors(cmd, checkDumpPrefix, checkDebug, IsTTY)
		if err := cc.Dump(v); err != nil {
			cmd.Println(errorMsg("ERROR checking the configuration file:") + fmt.Sprintf("\t%s\n", err.Error()))
			os.Exit(1) // skipcq: RVV-A0003
			return
		}
	}

	if checkRoutes {
		if RunRouterFunc == nil {
			cmd.Println(errorMsg("ERROR testing the configuration file:") + "\tthis build has no router to test against\n")
			os.Exit(1) // skipcq: RVV-A0003
			return
		}
		v.Debug = v.Debug || debug > 0
		if port != 0 {
			v.Port = port
		}
		if err := RunRouterFunc(v); err != nil {
			cmd.Println(errorMsg("ERROR testing the configuration file:") + fmt.Sprintf("\t%s\n", err.Error()))
			os.Exit(1) // skipcq: RVV-A0003
			return
		}
	}

	if IsTTY {
		cmd.Printf("%sSyntax OK!%s\n", dumper.ColorGreen, dumper.ColorReset)
		return
	}
	cmd.Println("Syntax OK!")
}

var CustomValidationFunc = func(_ config.ServiceConfig) []error {
	return nil
}

// RunRouterFunc registers the config's endpoints on a real router to catch
// route conflicts (`check --test-routes`). Each router variant sets it in main.
var RunRouterFunc func(cfg config.ServiceConfig) error

type SchemaHttpLoader http.Client

func (l *SchemaHttpLoader) Load(url string) (any, error) {
	client := (*http.Client)(l)
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("%s returned status code %d", url, resp.StatusCode)
	}

	body, err := jsonschema.UnmarshalJSON(resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}

	return body, resp.Body.Close()
}
