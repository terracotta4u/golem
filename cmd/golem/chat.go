package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/terracotta4u/golem/client"
	"github.com/terracotta4u/golem/daemon"
	"github.com/terracotta4u/golem/store"
)

// client import still needed for Send

func runChat() error {
	cfg, _, err := setup()
	if err != nil {
		return err
	}

	state, err := daemon.Ensure(cfg.Listen)
	if err != nil {
		return err
	}

	conv := store.New("cli")
	api := client.New(state.URL, state.Token)
	fmt.Printf("golem — %s — type a message, or /quit to exit\n", conv.ID)

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			fmt.Println()
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "/quit" || input == "/exit" {
			break
		}

		reply, err := api.Send(context.Background(), "cli", conv.ID, input, func(line string) {
			fmt.Println(line)
		})
		if reply != "" {
			fmt.Println(reply)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			continue
		}
	}
	return scanner.Err()
}
