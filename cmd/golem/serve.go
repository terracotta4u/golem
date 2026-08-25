package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"sort"

	"github.com/terracotta4u/golem/config"
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

	sup := supervisor.New(supervisor.Options{
		URL:      supervisor.URLFromListen(listen),
		Token:    token,
		Channels: channelList(app.cfg),
	})

	err := server.New(server.Options{
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

func channelList(cfg config.Config) []supervisor.Channel {
	names := make([]string, 0, len(cfg.Channels))
	for name := range cfg.Channels {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]supervisor.Channel, 0, len(names))
	for _, name := range names {
		ch := cfg.Channels[name]
		out = append(out, supervisor.Channel{
			Name:    name,
			Command: ch.Command,
			Args:    ch.Args,
			Env:     ch.Env,
		})
	}
	return out
}
