package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/extension"
	"github.com/terracotta4u/golem/release"
	"github.com/terracotta4u/golem/server"
	"github.com/terracotta4u/golem/supervisor"
)

const defaultListen = "127.0.0.1:8743"

func newServeCmd() *cobra.Command {
	var addr, token string
	cmd := &cobra.Command{
		Use:     "serve",
		Short:   "Start the Golem server",
		Long:    "Start the Golem HTTP server and run installed extensions.",
		Example: "  golem serve\n  golem serve --addr 127.0.0.1:9000",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(cmd, addr, token)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", defaultListen, "listen address")
	cmd.Flags().StringVar(&token, "token", "", "auth token (generated if empty)")
	_ = cmd.RegisterFlagCompletionFunc("addr", cobra.NoFileCompletions)
	_ = cmd.RegisterFlagCompletionFunc("token", cobra.NoFileCompletions)
	return cmd
}

func runServe(cmd *cobra.Command, addr, token string) error {
	app, err := loadApp()
	if err != nil {
		return err
	}
	defer app.conversations.Close()
	return serve(cmd.Context(), app, addr, token)
}

func serve(ctx context.Context, app *app, listen, token string) error {
	fmt.Fprint(os.Stderr, "The Golem has awoken.\n")
	if token == "" {
		token = server.NewToken()
	}
	fmt.Fprintf(os.Stderr, "Token: %s\n\n", token)

	extRoot, err := conf.ExtensionsDir()
	if err != nil {
		return err
	}
	exts, err := runningExtensions(app.cfg, extRoot)
	if err != nil {
		return err
	}
	sup := supervisor.New(supervisor.Options{
		URL:        supervisor.URLFromListen(listen),
		Token:      token,
		Extensions: exts,
	})

	err = server.New(server.Options{
		Agent:   app.agent,
		Store:   app.conversations,
		Addr:    listen,
		Token:   token,
		Hub:     app.hub,
		Version: version,
		Release: &release.Checker{Current: version},
		StartExtension: func(name string) error {
			cfg, _, err := conf.Load()
			if err != nil {
				return err
			}
			return startNamedExtension(cfg, extRoot, sup, name)
		},
		StopExtension: sup.Stop,
	}).Listen(ctx, func() {
		sup.Start(ctx)
	})
	sup.Wait()
	return err
}

func runningExtensions(cfg conf.Conf, extRoot string) ([]supervisor.Extension, error) {
	list, err := extension.List(extRoot)
	if err != nil {
		return nil, err
	}

	out := make([]supervisor.Extension, 0, len(list))
	for _, p := range list {
		ext, err := prepareExtension(cfg, p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "extension %s: %v\n", p.Name, err)
			continue
		}
		out = append(out, ext)
	}
	return out, nil
}

func startNamedExtension(cfg conf.Conf, extRoot string, sup *supervisor.Supervisor, name string) error {
	dir := filepath.Join(extRoot, name)
	p, err := extension.Load(dir)
	if err != nil {
		return err
	}
	if p.Name != name {
		return fmt.Errorf("extension %s: project name %q does not match", name, p.Name)
	}
	p.Dir = dir
	ext, err := prepareExtension(cfg, p)
	if err != nil {
		return err
	}
	return sup.Add(ext)
}

func prepareExtension(cfg conf.Conf, p extension.Project) (supervisor.Extension, error) {
	entry := cfg.Extensions[p.Name]
	if err := extension.EnsureVenv(p.Dir, p); err != nil {
		return supervisor.Extension{}, err
	}
	command, args, err := extension.ResolveCommand(p.Dir, p)
	if err != nil {
		return supervisor.Extension{}, err
	}
	return supervisor.Extension{
		Name:    p.Name,
		Command: command,
		Args:    args,
		Env:     entry.Env,
		Dir:     p.Dir,
	}, nil
}
