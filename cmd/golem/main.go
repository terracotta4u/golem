package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("golem", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	return runChat()
}
