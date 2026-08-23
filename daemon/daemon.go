package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"os"

	"github.com/terracotta4u/golem/config"
)

type State struct {
	PID   int    `json:"pid"`
	Addr  string `json:"addr"`
	URL   string `json:"url"`
	Token string `json:"token"`
}

func NewState(listen, token string) State {
	return State{
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

func Write(st State) error {
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

func Read() (State, error) {
	path, err := config.DaemonPath()
	if err != nil {
		return State{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return State{}, err
	}
	var st State
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, fmt.Errorf("parse daemon state: %w", err)
	}
	return st, nil
}

func Remove() {
	path, err := config.DaemonPath()
	if err != nil {
		return
	}
	_ = os.Remove(path)
}
