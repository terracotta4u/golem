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
