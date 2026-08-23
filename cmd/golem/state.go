package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/terracotta4u/golem/config"
)

type instanceState struct {
	URL   string `json:"url"`
	Token string `json:"token"`
}

func readState() (instanceState, error) {
	path, err := config.DaemonPath()
	if err != nil {
		return instanceState{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return instanceState{}, err
	}
	var st instanceState
	if err := json.Unmarshal(data, &st); err != nil {
		return instanceState{}, fmt.Errorf("parse instance state: %w", err)
	}
	return st, nil
}
