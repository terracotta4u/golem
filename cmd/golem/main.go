package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil && !errors.Is(err, context.Canceled) {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) > 0 && args[0] == "serve" {
		return runServe(args[1:])
	}

	fs := flag.NewFlagSet("golem", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	return runChat()
}
