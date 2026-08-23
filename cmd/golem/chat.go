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

func runChat() error {
	if _, _, err := setup(); err != nil {
		return err
	}

	state, err := daemon.Read()
	if err != nil || client.New(state.URL, state.Token).Health() != nil {
		return fmt.Errorf("golem is not running; start it with golem serve")
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
