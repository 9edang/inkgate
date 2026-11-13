package cmd

import (
	"encoding/base64"
	"fmt"
	"os"

	"github.com/9edang/inkgate/engine/config"
	"github.com/9edang/inkgate/engine/core"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

var IsTTY = isatty.IsTerminal(os.Stderr.Fd())

var (
	cfgFile              string
	debug                int
	port                 int
	checkRoutes          bool
	checkDebug           int
	lintCurrentSchema    bool
	lintCustomSchemaPath string
	lintNoNetwork        bool
	rawEmbedSchema       string
	rulesToExclude       string
	rulesToExcludePath   string
	severitiesToInclude  = "CRITICAL,HIGH,MEDIUM,LOW"
	formatTmpl           string
	parser               config.Parser
	run                  func(config.ServiceConfig)

	goSum           = "./go.sum"
	goVersion       = core.GoVersion
	libcVersion     = core.GlibcVersion
	checkDumpPrefix = "\t"
	gogetEnabled    = false

	DefaultRoot    Root
	RootCommand    Command
	RunCommand     Command
	CheckCommand   Command
	PluginCommand  Command
	VersionCommand Command
	AuditCommand   Command

	rootCmd = &cobra.Command{
		Use:   "inkgate",
		Short: "inkgate is a high-performance API gateway that helps you publish, secure, control, and monitor your services",
	}

	checkCmd = &cobra.Command{
		Use:     "check",
		Short:   "Validates that the configuration file is valid.",
		Long:    "Validates that the active configuration file has a valid syntax to run the service.\nChange the configuration file by using the --config flag",
		Run:     checkFunc,
		Aliases: []string{"validate"},
		Example: "inkgate check -d -l -c config.json",
	}

	runCmd = &cobra.Command{
		Use:     "run",
		Short:   "Runs the inkgate server.",
		Long:    "Runs the inkgate server.",
		Run:     runFunc,
		Example: "inkgate run -d -c config.json",
	}

	pluginCmd = &cobra.Command{
		Use:     "check-plugin",
		Short:   "Checks your plugin dependencies are compatible.",
		Long:    "Checks your plugin dependencies are compatible and proposes commands to update your dependencies.",
		Run:     pluginFunc,
		Example: "inkgate check-plugin -g 1.19.0 -s ./go.sum -f",
	}

	versionCmd = &cobra.Command{
		Use:     "version",
		Short:   "Shows inkgate version.",
		Long:    "Shows inkgate version.",
		Run:     versionFunc,
		Example: "inkgate version",
	}

	auditCmd = &cobra.Command{
		Use:     "audit",
		Short:   "Audits an inkgate configuration.",
		Long:    "Audits an inkgate configuration.",
		Run:     auditFunc,
		Example: "inkgate audit -i 1.1.1,1.1.2 -s CRITICAL -c inkgate.json",
	}
)

func init() {
	logo, err := base64.StdEncoding.DecodeString(encodedLogo)
	if err != nil {
		fmt.Println("decode error:", err)
	}

	cfgFlag := StringFlagBuilder(&cfgFile, "config", "c", "", "Path to the configuration file")
	debugFlag := CountFlagBuilder(&debug, "debug", "d", "Enables the debug endpoint")
	RootCommand = NewCommand(rootCmd)
	RootCommand.Cmd.SetHelpTemplate(string(logo) + "Version: " + core.InkgateVersion + "\n\n" + rootCmd.HelpTemplate())

	routesFlag := BoolFlagBuilder(&checkRoutes, "test-routes", "t", false, "Tests the endpoint patterns against the real router on the selected port")
	prefixFlag := StringFlagBuilder(&checkDumpPrefix, "indent", "i", checkDumpPrefix, "Indentation of the check dump")
	lintCurrentSchemaFlag := BoolFlagBuilder(&lintCurrentSchema, "lint", "l", lintCurrentSchema, "Enables the linting against the online JSON schema at INKGATE_SCHEMA_URL")
	lintCustomSchemaFlag := StringFlagBuilder(&lintCustomSchemaPath, "lint-schema", "s", lintCustomSchemaPath, "Lint against a custom schema path or URL")
	lintNoNetworkFlag := BoolFlagBuilder(&lintNoNetwork, "lint-no-network", "n", lintNoNetwork, "Lint against the builtin Inkgate JSON schema, no network is required")
	checkDebugFlag := CountFlagBuilder(&checkDebug, "debug", "d", "Information about how inkgate is interpreting your configuration file")
	allowUnknownFlag := BoolFlagBuilder(&allowUnknownNamespaces, "allow-unknown-namespaces", "", false,
		"Accepts extra_config namespaces inkgate does not know (they stay ignored), for configs migrated from KrakenD")
	CheckCommand = NewCommand(checkCmd, cfgFlag, checkDebugFlag, routesFlag, prefixFlag, lintCurrentSchemaFlag, lintCustomSchemaFlag, lintNoNetworkFlag, allowUnknownFlag)
	CheckCommand.AddConstraint(MutuallyExclusive("lint", "lint-no-network", "lint-schema"))

	portFlag := IntFlagBuilder(&port, "port", "p", 0, "Listening port for the http service")
	RunCommand = NewCommand(runCmd, cfgFlag, debugFlag, portFlag, allowUnknownFlag)

	goSumFlag := StringFlagBuilder(&goSum, "sum", "s", goSum, "Path to the go.sum file to analyze")
	goVersionFlag := StringFlagBuilder(&goVersion, "go", "g", goVersion, "The version of the go compiler used for your plugin")
	libcVersionFlag := StringFlagBuilder(&libcVersion, "libc", "l", "", "Version of the libc library used")
	gogetFlag := BoolFlagBuilder(&gogetEnabled, "format", "f", false, "Shows fix commands to update your dependencies")
	PluginCommand = NewCommand(pluginCmd, goSumFlag, goVersionFlag, libcVersionFlag, gogetFlag)

	rulesToExcludeFlag := StringFlagBuilder(&rulesToExclude, "ignore", "i", rulesToExclude, "List of rules to ignore (comma-separated, no spaces)")
	severitiesToIncludeFlag := StringFlagBuilder(&severitiesToInclude, "severity", "s", severitiesToInclude, "List of severities to include (comma-separated, no spaces)")
	pathToRulesToExcludeFlag := StringFlagBuilder(&rulesToExcludePath, "ignore-file", "I", rulesToExcludePath, "Path to a text-plain file containing the list of rules to exclude")
	formatFlag := StringFlagBuilder(&formatTmpl, "format", "f", formatTmpl, "Inline go template to render the results")
	AuditCommand = NewCommand(auditCmd, cfgFlag, rulesToExcludeFlag, severitiesToIncludeFlag, pathToRulesToExcludeFlag, formatFlag)

	VersionCommand = NewCommand(versionCmd)

	DefaultRoot = NewRoot(RootCommand, CheckCommand, RunCommand, PluginCommand, VersionCommand, AuditCommand)
}

const encodedLogo = "ICBfICAgICAgIF8gICAgICAgICAgICAgICAgIF8KIChfKV8gX18gfCB8IF9fIF9fIF8gIF9fIF98IHxfIF9fXwogfCB8ICdfIFx8IHwvIC8vIF9gIHwvIF9gIHwgX18vIF8gXAogfCB8IHwgfCB8ICAgPHwgKF98IHwgKF98IHwgfHwgIF9fLwogfF98X3wgfF98X3xcX1xcX18sIHxcX18sX3xcX19cX19ffAogICAgICAgICAgICAgICB8X19fLwo="
