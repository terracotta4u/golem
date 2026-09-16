package main

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/extension"
)

func newExtensionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:        "extension",
		Short:      "Manage extensions",
		SuggestFor: []string{"extensions"},
	}
	cmd.AddCommand(newExtensionAddCmd(), newExtensionListCmd(), newExtensionRemoveCmd())
	return cmd
}

func newExtensionAddCmd() *cobra.Command {
	var force bool
	var ref string
	cmd := &cobra.Command{
		Use:   "add SOURCE",
		Short: "Install an extension from a path, zip, or GitHub URL",
		Example: `  golem extension add ./echo
  golem extension add --ref v1.2.0 https://github.com/terracotta4u/golem-telegram`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtensionAdd(cmd, args[0], force, ref)
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "replace an existing install")
	cmd.Flags().StringVar(&ref, "ref", "", "git ref for a GitHub URL (default HEAD)")
	_ = cmd.RegisterFlagCompletionFunc("ref", cobra.NoFileCompletions)
	return cmd
}

func newExtensionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed extensions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtensionList(cmd)
		},
	}
}

func newExtensionRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:               "remove NAME",
		Short:             "Uninstall an extension",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completeInstalledExtensions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runExtensionRemove(cmd, args[0])
		},
	}
}

func completeInstalledExtensions(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
	if len(args) != 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	root, err := conf.ExtensionsDir()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	list, err := extension.List(root)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}
	names := make([]cobra.Completion, 0, len(list))
	for _, p := range list {
		names = append(names, p.Name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

func runExtensionAdd(cmd *cobra.Command, source string, force bool, ref string) error {
	destRoot, err := conf.ExtensionsDir()
	if err != nil {
		return err
	}
	p, err := extension.Install(source, destRoot, extension.Options{
		Force: force,
		Ref:   ref,
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
	fmt.Fprintf(cmd.ErrOrStderr(), "installed %s\n", p.Name)
	return nil
}

func runExtensionList(cmd *cobra.Command) error {
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
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	for _, p := range list {
		fmt.Fprintf(w, "%s\t%s\t%s\n", p.Name, p.Version, cfg.Extensions[p.Name].Source)
	}
	return w.Flush()
}

func runExtensionRemove(cmd *cobra.Command, name string) error {
	root, err := conf.ExtensionsDir()
	if err != nil {
		return err
	}
	if err := extension.Remove(root, name); err != nil {
		return err
	}
	cfg, _, err := conf.Load()
	if err != nil {
		return err
	}
	conf.RemoveExtension(&cfg, name)
	if err := conf.Save(cfg); err != nil {
		return err
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "removed %s\n", name)
	return nil
}
