package main

import (
	"fmt"
	"os"
	"runtime"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func runVersion(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: golem version")
	}
	fmt.Fprintf(os.Stdout, "golem %s %s/%s\ncommit: %s\nbuilt: %s\n", version, runtime.GOOS, runtime.GOARCH, commit, date)
	return nil
}
