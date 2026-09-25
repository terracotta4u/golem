package web_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/tool"
)

func getTurnEvents(t *testing.T, base, token, id string) []sseEvent {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, base+"/v1/turns/"+id, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("get events status = %d: %s", resp.StatusCode, b)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("Content-Type = %q, want text/event-stream", ct)
	}
	return readSSE(t, resp.Body)
}

type sseEvent struct {
	Event string
	Data  string
}

func readSSE(t *testing.T, r io.Reader) []sseEvent {
	t.Helper()
	var events []sseEvent
	sc := bufio.NewScanner(r)
	var event, data string
	var hasData bool
	flush := func() {
		if event == "" && !hasData {
			return
		}
		events = append(events, sseEvent{Event: event, Data: data})
		event, data, hasData = "", "", false
	}
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "event:"):
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		case strings.HasPrefix(line, "data:"):
			payload := strings.TrimPrefix(strings.TrimPrefix(line, "data:"), " ")
			if hasData {
				data += "\n"
			}
			data += payload
			hasData = true
		case line == "":
			flush()
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatal(err)
	}
	flush()
	return events
}

type replyProvider struct {
	text string
}

func (p *replyProvider) Chat(_ context.Context, _ provider.ChatRequest) (provider.Message, error) {
	return provider.Message{Role: "assistant", Content: p.text}, nil
}

type gateProvider struct {
	waiting chan struct{}
	release chan struct{}
	text    string
}

func (p *gateProvider) Chat(ctx context.Context, _ provider.ChatRequest) (provider.Message, error) {
	select {
	case p.waiting <- struct{}{}:
	case <-ctx.Done():
		return provider.Message{}, ctx.Err()
	}
	select {
	case <-p.release:
	case <-ctx.Done():
		return provider.Message{}, ctx.Err()
	}
	return provider.Message{Role: "assistant", Content: p.text}, nil
}

type gatedScript struct {
	replies []provider.Message
	gateAt  int
	waiting chan struct{}
	release chan struct{}
	i       int
}

func (p *gatedScript) Chat(ctx context.Context, _ provider.ChatRequest) (provider.Message, error) {
	if p.i >= len(p.replies) {
		return provider.Message{}, errors.New("unexpected Chat call")
	}
	i := p.i
	p.i++
	if i == p.gateAt {
		select {
		case p.waiting <- struct{}{}:
		case <-ctx.Done():
			return provider.Message{}, ctx.Err()
		}
		select {
		case <-p.release:
		case <-ctx.Done():
			return provider.Message{}, ctx.Err()
		}
	}
	return p.replies[i], nil
}

type stubTool struct {
	name   string
	result string
}

func (t *stubTool) Spec() tool.Spec {
	return tool.Spec{Name: t.name, Description: "stub"}
}

func (t *stubTool) Call(_ context.Context, _ json.RawMessage) (string, error) {
	return t.result, nil
}

type errProvider struct {
	err error
}

func (p *errProvider) Chat(_ context.Context, _ provider.ChatRequest) (provider.Message, error) {
	return provider.Message{}, p.err
}
