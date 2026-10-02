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
	var addr, publicURL, token string
	cmd := &cobra.Command{
		Use:     "serve",
		Short:   "Start the Golem server",
		Long:    "Start the Golem HTTP server and run installed extensions.",
		Example: "  golem serve\n  golem serve --addr 192.168.1.20:8743\n  golem serve --url https://golem.example.com",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(cmd, addr, publicURL, token)
		},
	}
	cmd.Flags().StringVar(&addr, "addr", defaultListen, "listen address (loopback, IP, or wildcard)")
	cmd.Flags().StringVar(&publicURL, "url", "", "public origin when the UI is reachable beyond this machine")
	cmd.Flags().StringVar(&token, "token", "", "auth token (generated if empty)")
	_ = cmd.RegisterFlagCompletionFunc("addr", cobra.NoFileCompletions)
	_ = cmd.RegisterFlagCompletionFunc("url", cobra.NoFileCompletions)
	_ = cmd.RegisterFlagCompletionFunc("token", cobra.NoFileCompletions)
	return cmd
}

func runServe(cmd *cobra.Command, addr, publicURL, token string) error {
	app, err := loadApp()
	if err != nil {
		return err
	}
	defer app.conversations.Close()
	return serve(cmd.Context(), app, addr, publicURL, token)
}

func serve(ctx context.Context, app *app, listen, publicURL, token string) error {
	// Held until this process exits so CLI remove and force-add can see that
	// extension children belong to a running server.
	unlock, err := conf.HoldServe()
	if err != nil {
		return err
	}
	defer unlock()

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
		URL:       supervisor.URLFromListen(listen),
		Token:     token,
		Processes: exts,
		OnExit:    app.reg.Drop,
	})

	app.reg.SetToken(token)
	err = server.New(server.Options{
		Agent:    app.agent,
		Store:    app.conversations,
		Addr:     listen,
		URL:      publicURL,
		Token:    token,
		Registry: app.reg,
		Version:  version,
		Release:  &release.Checker{Current: version},
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

func runningExtensions(cfg conf.Conf, extRoot string) ([]supervisor.Process, error) {
	list, err := extension.List(extRoot)
	if err != nil {
		return nil, err
	}

	out := make([]supervisor.Process, 0, len(list))
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

func prepareExtension(cfg conf.Conf, p extension.Project) (supervisor.Process, error) {
	entry := cfg.Extensions[p.Name]
	if err := extension.EnsureVenv(p.Dir, p); err != nil {
		return supervisor.Process{}, err
	}
	command, args, err := extension.ResolveCommand(p.Dir, p)
	if err != nil {
		return supervisor.Process{}, err
	}
	return supervisor.Process{
		Name:    p.Name,
		Command: command,
		Args:    args,
		Env:     entry.Env,
		Dir:     p.Dir,
	}, nil
}
