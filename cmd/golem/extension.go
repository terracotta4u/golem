package main

import (
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/extension"
)

func runExtension(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: golem extension add <source> | golem extension list | golem extension remove <name>")
	}
	switch args[0] {
	case "add":
		return runExtensionAdd(args[1:])
	case "list":
		return runExtensionList(args[1:])
	case "remove":
		return runExtensionRemove(args[1:])
	default:
		return fmt.Errorf("usage: golem extension add <source> | golem extension list | golem extension remove <name>")
	}
}

func runExtensionAdd(args []string) error {
	fs := flag.NewFlagSet("golem extension add", flag.ContinueOnError)
	force := fs.Bool("force", false, "replace an existing install")
	ref := fs.String("ref", "", "git ref for a GitHub URL (default HEAD)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return fmt.Errorf("usage: golem extension add <source>")
	}

	destRoot, err := conf.ExtensionsDir()
	if err != nil {
		return err
	}
	p, err := extension.Install(fs.Arg(0), destRoot, extension.Options{
		Force: *force,
		Ref:   *ref,
	})
	if err != nil {
		return err
	}
	cfg, _, err := conf.Load()
	if err != nil {
		return err
	}
	conf.SetExtensionOrigin(&cfg, p.Name, p.Origin.Source, p.Origin.Ref, p.Origin.Revision)
	if err := conf.Save(cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "installed %s\n", p.Name)
	return nil
}

func runExtensionList(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("usage: golem extension list")
	}
	root, err := conf.ExtensionsDir()
	if err != nil {
		return err
	}
	list, err := extension.List(root)
	if err != nil {
		return err
	}
	cfg, _, err := conf.Load()
	if err != nil {
		return err
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	for _, p := range list {
		fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, p.Version, cfg.Extensions[p.Name].Source)
	}
	return w.Flush()
}

func runExtensionRemove(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: golem extension remove <name>")
	}
	root, err := conf.ExtensionsDir()
	if err != nil {
		return err
	}
	if err := extension.Remove(root, args[0]); err != nil {
		return err
	}
	cfg, _, err := conf.Load()
	if err != nil {
		return err
	}
	conf.RemoveExtension(&cfg, args[0])
	if err := conf.Save(cfg); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "removed %s\n", args[0])
	return nil
}
