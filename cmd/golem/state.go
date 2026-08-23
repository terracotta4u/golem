package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/terracotta4u/golem/config"
)

type instanceState struct {
	PID   int    `json:"pid"`
	Addr  string `json:"addr"`
	URL   string `json:"url"`
	Token string `json:"token"`
}

func newState(listen, token string) instanceState {
	return instanceState{
		PID:   os.Getpid(),
		Addr:  listen,
		URL:   urlFromListen(listen),
		Token: token,
	}
}

func urlFromListen(listen string) string {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return "http://" + listen
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port)
}

func writeState(st instanceState) error {
	path, err := config.DaemonPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
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

func removeState() {
	path, err := config.DaemonPath()
	if err != nil {
		return
	}
	_ = os.Remove(path)
}
