package main

import (
	"fmt"

	"github.com/terracotta4u/golem/client"
	"github.com/terracotta4u/golem/config"
)

func runChat() error {
	if _, _, err := config.Load(); err != nil {
		return err
	}

	state, err := readState()
	if err != nil || client.New(state.URL, state.Token).Health() != nil {
		return fmt.Errorf("golem is not running; start it with golem serve")
	}
	return nil
}
