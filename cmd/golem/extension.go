package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/terracotta4u/golem/config"
	"github.com/terracotta4u/golem/extension"
)

func runExtension(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: golem extension add <path>")
	}
	switch args[0] {
	case "add":
		return runExtensionAdd(args[1:])
	default:
		return fmt.Errorf("usage: golem extension add <path>")
	}
}

func runExtensionAdd(args []string) error {
	fs := flag.NewFlagSet("golem extension add", flag.ContinueOnError)
	force := fs.Bool("force", false, "replace an existing install")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: golem extension add <path>")
	}

	destRoot, err := config.ExtensionsDir()
	if err != nil {
		return err
	}
	m, err := extension.Install(fs.Arg(0), destRoot, *force)
	if err != nil {
		return err
	}

	cfg, _, err := config.Load()
	if err != nil {
		return err
	}
	config.ScaffoldExtension(&cfg, m.Name, m.Env)
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "installed %s\n", m.Name)
	return nil
}
