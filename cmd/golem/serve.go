package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/terracotta4u/golem/conf"
	"github.com/terracotta4u/golem/extension"
	"github.com/terracotta4u/golem/server"
	"github.com/terracotta4u/golem/supervisor"
)

func runServe(args []string) error {
	fs := flag.NewFlagSet("golem", flag.ContinueOnError)
	addr := fs.String("addr", "", "listen address (default from conf)")
	if err := fs.Parse(args); err != nil {
		return err
	}

	app, err := loadApp()
	if err != nil {
		return err
	}

	listen := *addr
	if listen == "" {
		listen = app.cfg.Listen
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	return serve(ctx, app, listen)
}

func serve(ctx context.Context, app *app, listen string) error {
	token := server.NewToken()
	fmt.Fprintf(os.Stderr, "token: %s\n", token)

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
		Agent: app.agent,
		Store: app.store,
		Addr:  listen,
		Token: token,
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
		entry := cfg.Extensions[p.Name]
		if !entry.IsEnabled() {
			continue
		}
		if err := extension.EnsureVenv(p.Dir, p); err != nil {
			return nil, err
		}
		command, err := extension.ResolveCommand(p.Dir, p)
		if err != nil {
			return nil, err
		}
		out = append(out, supervisor.Extension{
			Name:    p.Name,
			Command: command,
			Env:     entry.Env,
			Dir:     p.Dir,
		})
	}
	return out, nil
}
