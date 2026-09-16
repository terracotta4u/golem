package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runVersion(cmd)
		},
	}
}

func runVersion(cmd *cobra.Command) error {
	_, err := fmt.Fprint(cmd.OutOrStdout(), versionInfo())
	return err
}

func versionInfo() string {
	return fmt.Sprintf("golem %s %s/%s\ncommit: %s\nbuilt: %s\n", version, runtime.GOOS, runtime.GOARCH, commit, date)
}
