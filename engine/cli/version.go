package cmd

import (
	"github.com/9edang/inkgate/engine/core"
	"github.com/spf13/cobra"
)

func versionFunc(cmd *cobra.Command, _ []string) {
	cmd.Println("inkgate version:", core.InkgateVersion)
	cmd.Println("Go Version:", core.GoVersion)
	cmd.Println("Glibc Version:", core.GlibcVersion)
}
