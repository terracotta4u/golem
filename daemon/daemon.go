package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/terracotta4u/golem/client"
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

func Listening(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

var startFn = start

func Ensure(listen string) (State, error) {
	if Listening(listen) {
		st, err := Read()
		if err != nil {
			return State{}, fmt.Errorf("golem is listening on %s but %v; stop it and run golem serve", listen, err)
		}
		if err := client.New(st.URL, st.Token).Health(); err != nil {
			return State{}, fmt.Errorf("golem is running but unreachable: %w", err)
		}
		return st, nil
	}

	if err := startFn(); err != nil {
		return State{}, fmt.Errorf("start golem: %w", err)
	}

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if Listening(listen) {
			st, err := Read()
			if err == nil && client.New(st.URL, st.Token).Health() == nil {
				return st, nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}

	logPath, _ := config.LogPath()
	return State{}, fmt.Errorf("timed out waiting for golem; see %s", logPath)
}

func start() error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("executable: %w", err)
	}
	logPath, err := config.LogPath()
	if err != nil {
		return err
	}
	logf, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("open log: %w", err)
	}
	defer logf.Close()

	cmd := exec.Command(exe, "serve")
	cmd.Stdout = logf
	cmd.Stderr = logf
	detach(cmd)
	if err := cmd.Start(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "started golem (logs: %s)\n", logPath)
	return nil
}
