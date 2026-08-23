package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	"github.com/terracotta4u/golem/server"
)

func runServe(args []string) error {
	fs := flag.NewFlagSet("golem serve", flag.ContinueOnError)
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

	token := server.NewToken()
	fmt.Fprintf(os.Stderr, "token: %s\n", token)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	wroteState := false
	err = server.New(server.Options{
		Agent: app.agent,
		Store: app.store,
		Addr:  listen,
		Token: token,
	}).Listen(ctx, func() {
		if err := writeState(newState(listen, token)); err != nil {
			fmt.Fprintf(os.Stderr, "instance state: %v\n", err)
		} else {
			wroteState = true
		}
	})
	if wroteState {
		removeState()
	}
	return err
}
