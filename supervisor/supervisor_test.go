package supervisor

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStartEmptyDoesNothing(t *testing.T) {
	s := New(Options{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	s.Start(ctx)
	done := make(chan struct{})
	go func() {
		s.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatal("Wait did not return for an empty channel list")
	}
}

func TestStartInjectsEnv(t *testing.T) {
	out := filepath.Join(t.TempDir(), "env")
	s := New(Options{
		URL:   "http://127.0.0.1:8743",
		Token: "secret",
		Channels: []Channel{{
			Name:    "echo",
			Command: "sh",
			Args:    []string{"-c", "printf '%s %s' \"$GOLEM_URL\" \"$GOLEM_TOKEN\" > " + strconv.Quote(out)},
		}},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	s.Start(ctx)
	deadline := time.Now().Add(time.Second)
	for {
		if _, err := os.Stat(out); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child did not write env")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	s.Wait()

	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.TrimSpace(string(data))
	if got != "http://127.0.0.1:8743 secret" {
		t.Fatalf("child env = %q, want GOLEM_URL and GOLEM_TOKEN", got)
	}
}

func TestStartRestartsExitedChild(t *testing.T) {
	out := filepath.Join(t.TempDir(), "runs")
	s := New(Options{
		Channels: []Channel{{
			Name:    "echo",
			Command: "sh",
			Args:    []string{"-c", "echo x >> " + strconv.Quote(out)},
		}},
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s.Start(ctx)

	deadline := time.Now().Add(5 * time.Second)
	for {
		data, _ := os.ReadFile(out)
		if strings.Count(string(data), "x") >= 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("child was not restarted after it exited")
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel()
	s.Wait()
}
