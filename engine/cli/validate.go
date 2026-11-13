// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/9edang/inkgate/engine/config"
)

// KnownNamespaces are the extra_config namespaces the build supports, as written in a config file (aliases and code
// names). flow/boot sets it. While it is empty, namespaces are not checked.
var KnownNamespaces []string

// allowUnknownNamespaces is --allow-unknown-namespaces: for configs migrated from KrakenD, which carry namespaces
// inkgate does not read.
var allowUnknownNamespaces bool

// CustomValidationFunc is the extra validation a build adds on top of the namespace check, for example options a
// feature only reads when it builds its endpoint. Both `run` and `check` call it.
var CustomValidationFunc = func(_ config.ServiceConfig) []error {
	return nil
}

// validateConfig returns what is wrong with a parsed config, and a hint on how to go on when that is only unknown
// namespaces.
func validateConfig(cfg config.ServiceConfig) (errs []error, hint string) {
	if len(KnownNamespaces) > 0 && !allowUnknownNamespaces {
		if unknown := cfg.ValidateNamespaces(KnownNamespaces); len(unknown) > 0 {
			errs = append(errs, unknown...)
			hint = "A namespace inkgate does not read is ignored, so its feature would be off. Fix the name, or pass " +
				"--allow-unknown-namespaces for a config migrated from KrakenD."
		}
	}

	return append(errs, CustomValidationFunc(cfg)...), hint
}

// exitOnInvalidConfig prints what validateConfig found and exits with status 1.
func exitOnInvalidConfig(cmd *cobra.Command, cfg config.ServiceConfig) {
	errs, hint := validateConfig(cfg)
	if len(errs) == 0 {
		return
	}
	var b strings.Builder
	for _, err := range errs {
		fmt.Fprintf(&b, "\t%s\n", err.Error())
	}
	if hint != "" {
		fmt.Fprintf(&b, "\n%s\n", hint)
	}
	cmd.Println(errorMsg("ERROR validating the configuration file:\n") + b.String())
	os.Exit(1) // skipcq: RVV-A0003
}
