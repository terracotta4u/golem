package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/terracotta4u/golem/config"
	"github.com/terracotta4u/golem/extension"
)

func runExtension(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: golem extension add <path> | golem extension list | golem extension remove <name>")
	}
	switch args[0] {
	case "add":
		return runExtensionAdd(args[1:])
	case "list":
		return runExtensionList(args[1:])
	case "remove":
		return runExtensionRemove(args[1:])
	default:
		return fmt.Errorf("usage: golem extension add <path> | golem extension list | golem extension remove <name>")
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

func runExtensionList(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: golem extension list")
	}
	root, err := config.ExtensionsDir()
	if err != nil {
		return err
	}
	list, err := extension.List(root)
	if err != nil {
		return err
	}
	cfg, _, err := config.Load()
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, m := range list {
		status := "disabled"
		if entry, ok := cfg.Extensions[m.Name]; ok && entry.IsEnabled() {
			status = "enabled"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", m.Name, m.Version, m.Kind, status)
	}
	return w.Flush()
}

func runExtensionRemove(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: golem extension remove <name>")
	}
	root, err := config.ExtensionsDir()
	if err != nil {
		return err
	}
	if err := extension.Remove(root, args[0]); err != nil {
		return err
	}
	cfg, _, err := config.Load()
	if err != nil {
		return err
	}
	config.RemoveExtension(&cfg, args[0])
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "removed %s\n", args[0])
	return nil
}
