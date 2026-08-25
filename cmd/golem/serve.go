package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"

	"github.com/terracotta4u/golem/config"
	"github.com/terracotta4u/golem/extension"
	"github.com/terracotta4u/golem/server"
	"github.com/terracotta4u/golem/supervisor"
)

func runServe(args []string) error {
	fs := flag.NewFlagSet("golem", flag.ContinueOnError)
	addr := fs.String("addr", "", "listen address (default from config)")
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

	extRoot, err := config.ExtensionsDir()
	if err != nil {
		return err
	}
	exts, err := extensionList(app.cfg, extRoot)
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

func extensionList(cfg config.Config, extRoot string) ([]supervisor.Extension, error) {
	names := make([]string, 0, len(cfg.Channels))
	for name := range cfg.Channels {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]supervisor.Extension, 0, len(names))
	for _, name := range names {
		ch := cfg.Channels[name]
		ext := supervisor.Extension{
			Name:    name,
			Command: ch.Command,
			Args:    ch.Args,
			Env:     ch.Env,
		}
		dir := filepath.Join(extRoot, name)
		m, err := extension.Load(dir)
		if err == nil {
			if m.Name != name {
				return nil, fmt.Errorf("extension %s: manifest name %q does not match", name, m.Name)
			}
			if ext.Command == "" {
				ext.Command = m.Command
			}
			if ext.Args == nil {
				ext.Args = m.Args
			}
			ext.Dir = dir
		} else if !os.IsNotExist(err) {
			return nil, err
		}
		out = append(out, ext)
	}
	return out, nil
}
