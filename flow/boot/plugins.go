package boot

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/9edang/inkgate/context"

	"github.com/spf13/cobra"

	cmd "github.com/9edang/inkgate/engine/cli"
	"github.com/9edang/inkgate/engine/logging"
	proxy "github.com/9edang/inkgate/engine/proxy/plugin"
	client "github.com/9edang/inkgate/engine/transport/http/client/plugin"
	server "github.com/9edang/inkgate/engine/transport/http/server/plugin"
)

// LoadPluginsWithContext loads and registers the plugins so they can be used if enabled at the configuration.
func LoadPluginsWithContext(ctx context.Context, folder, pattern string, logger logging.Logger) {
	logger.Debug("[SERVICE: Plugin Loader] Starting loading process")

	n, err := client.LoadWithLogger(
		folder,
		pattern,
		client.RegisterClient,
		logger,
	)
	logPluginLoaderErrors(logger, "[SERVICE: Executor Plugin]", n, err)

	n, err = server.LoadWithLogger(
		folder,
		pattern,
		server.RegisterHandler,
		logger,
	)
	logPluginLoaderErrors(logger, "[SERVICE: Handler Plugin]", n, err)

	n, err = proxy.LoadWithLoggerAndContext(
		ctx,
		folder,
		pattern,
		proxy.RegisterModifier,
		logger,
	)
	logPluginLoaderErrors(logger, "[SERVICE: Modifier Plugin]", n, err)

	logger.Debug("[SERVICE: Plugin Loader] Loading process completed")
}

func logPluginLoaderErrors(logger logging.Logger, tag string, n int, err error) {
	if err != nil {
		if mErrs, ok := err.(pluginLoaderErr); ok {
			for _, err := range mErrs.Errs() {
				logger.Debug(tag, err.Error())
			}
		} else {
			logger.Debug(tag, err.Error())
		}
	}
	if n > 0 {
		logger.Info(tag, "Total plugins loaded:", n)
	}
}

type pluginLoaderErr interface {
	Errs() []error
}

var (
	serverExpected   bool
	clientExpected   bool
	modifierExpected bool

	testPluginCmd = &cobra.Command{
		Use:     "test-plugin [flags] [artifacts]",
		Short:   "Tests that one or more plugins are loadable into Inkgate.",
		Run:     testPluginFunc,
		Example: "inkgate test-plugin -scm ./plugins/my_plugin.so ./plugins/my_other_plugin.so",
	}

	serverExpectedFlag   cmd.FlagBuilder
	clientExpectedFlag   cmd.FlagBuilder
	modifierExpectedFlag cmd.FlagBuilder

	additionalChecks []func(context.Context, *cobra.Command, string, string) bool

	reLogErrorPlugins = regexp.MustCompile(`(?m)plugin \#\d+ \(.*\): (.*)`)
)

func init() {
	serverExpectedFlag = cmd.BoolFlagBuilder(&serverExpected, "server", "s", false, "The artifact should contain a Server Plugin.")
	clientExpectedFlag = cmd.BoolFlagBuilder(&clientExpected, "client", "c", false, "The artifact should contain a Client Plugin.")
	modifierExpectedFlag = cmd.BoolFlagBuilder(&modifierExpected, "modifier", "m", false, "The artifact should contain a Req/Resp Modifier Plugin.")
}

func NewTestPluginCmd() cmd.Command {
	return cmd.NewCommand(testPluginCmd, serverExpectedFlag, clientExpectedFlag, modifierExpectedFlag)
}

func NewTestPluginCmdWithArgs(flags ...cmd.FlagBuilder) cmd.Command {
	c := NewTestPluginCmd()
	for _, f := range flags {
		c.AddFlag(f)
	}

	return c
}

func AddCheck(f func(context.Context, *cobra.Command, string, string) bool) {
	additionalChecks = append(additionalChecks, f)
}

//nolint:cyclop,gocyclo // long carried-over function, split when it is next changed
func testPluginFunc(ccmd *cobra.Command, args []string) { // skipcq: GO-R1005
	if len(args) == 0 {
		ccmd.Println("At least one plugin is required.")
		os.Exit(1)
	}
	if !serverExpected && !clientExpected && !modifierExpected && len(additionalChecks) == 0 {
		ccmd.Println("You must declare the expected type of the plugin.")
		os.Exit(1)
	}

	start := time.Now()

	ctx, cancel := context.WithCancel(context.Background())

	var failed int
	globalOK := true
	for _, pluginPath := range args {
		// #nosec G304 -- path comes from the operator config or the CLI, not from request data
		f, err := os.Open(pluginPath)
		if os.IsNotExist(err) {
			ccmd.Println(fmt.Sprintf("[KO] Unable to open the plugin %s.", pluginPath))
			failed++
			globalOK = false

			continue
		}
		// #nosec G104 -- closing is best effort, the error is not actionable
		f.Close()

		name := filepath.Base(pluginPath)
		folder := filepath.Dir(pluginPath)
		ok := true

		if serverExpected {
			ok = checkHandlerPlugin(ccmd, folder, name) && ok
		}

		if modifierExpected {
			ok = checkModifierPlugin(ctx, ccmd, folder, name) && ok
		}

		if clientExpected {
			ok = checkClientPlugin(ccmd, folder, name) && ok
		}

		for _, check := range additionalChecks {
			ok = check(ctx, ccmd, folder, name) && ok
		}

		if !ok {
			failed++
		}

		globalOK = globalOK && ok
	}

	cancel()

	if !globalOK {
		ccmd.Println(fmt.Sprintf("[KO] %d tested plugin(s) in %s.\n%d plugin(s) failed.", len(args), time.Since(start), failed))
		os.Exit(1)
	}

	ccmd.Println(fmt.Sprintf("[OK] %d tested plugin(s) in %s", len(args), time.Since(start)))
}

//nolint:dupl // the handler, client and modifier checks are alike on purpose
func checkClientPlugin(ccmd *cobra.Command, folder, name string) bool {
	_, err := client.LoadWithLogger(
		folder,
		name,
		client.RegisterClient,
		logging.NoOp,
	)
	if err == nil {
		ccmd.Println("[OK] CLIENT\t" + name)
		return true
	}

	var msg string
	if mErrs, ok := err.(pluginLoaderErr); ok {
		var msgSb193 strings.Builder
		for _, err := range mErrs.Errs() {
			msgSb193.WriteString(err.Error())
		}
		msg += msgSb193.String()
	} else {
		msg = err.Error()
	}

	if strings.Contains(msg, "symbol ClientRegisterer not found") {
		ccmd.Println(fmt.Sprintf("[KO] CLIENT\t%s: The plugin does not contain a ClientRegisterer.", name))
		return false
	}

	for _, match := range reLogErrorPlugins.FindAllStringSubmatch(msg, -1) {
		msg = match[1]
	}

	ccmd.Println(fmt.Sprintf("[KO] CLIENT\t%s: %s", name, msg))

	return false
}

//nolint:dupl // the handler, client and modifier checks are alike on purpose
func checkHandlerPlugin(ccmd *cobra.Command, folder, name string) bool {
	_, err := server.LoadWithLogger(
		folder,
		name,
		server.RegisterHandler,
		logging.NoOp,
	)
	if err == nil {
		ccmd.Println("[OK] SERVER\t" + name)
		return true
	}

	var msg string
	if mErrs, ok := err.(pluginLoaderErr); ok {
		var msgSb227 strings.Builder
		for _, err := range mErrs.Errs() {
			msgSb227.WriteString(err.Error())
		}
		msg += msgSb227.String()
	} else {
		msg = err.Error()
	}

	if strings.Contains(msg, "symbol HandlerRegisterer not found") {
		ccmd.Println(fmt.Sprintf("[KO] SERVER\t%s: The plugin does not contain a HandlerRegisterer.", name))
		return false
	}

	for _, match := range reLogErrorPlugins.FindAllStringSubmatch(msg, -1) {
		msg = match[1]
	}

	ccmd.Println(fmt.Sprintf("[KO] SERVER\t%s: %s", name, msg))

	return false
}

func checkModifierPlugin(ctx context.Context, ccmd *cobra.Command, folder, name string) bool {
	_, err := proxy.LoadWithLoggerAndContext(
		ctx,
		folder,
		name,
		proxy.RegisterModifier,
		logging.NoOp,
	)
	if err == nil {
		ccmd.Println("[OK] MODIFIER\t" + name)
		return true
	}

	var msg string
	if mErrs, ok := err.(pluginLoaderErr); ok {
		var msgSb262 strings.Builder
		for _, err := range mErrs.Errs() {
			msgSb262.WriteString(err.Error())
		}
		msg += msgSb262.String()
	} else {
		msg = err.Error()
	}

	if strings.Contains(msg, "symbol ModifierRegisterer not found") {
		ccmd.Println(fmt.Sprintf("[KO] MODIFIER\t%s: The plugin does not contain a ModifierRegisterer.", name))
		return false
	}

	for _, match := range reLogErrorPlugins.FindAllStringSubmatch(msg, -1) {
		msg = match[1]
	}

	ccmd.Println(fmt.Sprintf("[KO] MODIFIER\t%s: %s", name, msg))

	return false
}
