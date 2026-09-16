package main

import (
	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "golem",
		Short:         "An extensible personal AI agent",
		Long:          "Golem is an extensible personal AI agent that runs on your machine.",
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.SetVersionTemplate(versionInfo())
	cmd.AddCommand(newServeCmd(), newVersionCmd(), newExtensionCmd())
	return cmd
}
