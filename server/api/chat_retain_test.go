package api

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/terracotta4u/golem/agent"
	"github.com/terracotta4u/golem/conversation"
	"github.com/terracotta4u/golem/provider"
)

func TestSendEventDeliversTerminalWhenBufferIsFull(t *testing.T) {
	ch := make(chan Event, subBuf)
	for i := range subBuf {
		sendEvent([]chan Event{ch}, Event{Name: "log", Text: strconv.Itoa(i)})
	}
	sendEvent([]chan Event{ch}, Event{Name: "done", Text: "finished"})

	var last Event
	for range subBuf {
		select {
		case ev := <-ch:
			last = ev
		default:
			t.Fatal("buffer was not full")
		}
	}
	if last.Name != "done" || last.Text != "finished" {
		t.Fatalf("last = %+v, want done", last)
	}
	select {
	case ev := <-ch:
		t.Fatalf("extra event %+v", ev)
	default:
	}
}

func TestCompletedTurnExpires(t *testing.T) {
	st, err := conversation.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	c := NewChat(agent.New(staticReply("hi")), st)
	c.retain = 200 * time.Millisecond
	id := c.Start(context.Background(), "conv-1", "cli", "hello")

	deadline := time.Now().Add(2 * time.Second)
	for {
		c.mu.Lock()
		turn, ok := c.turns[id]
		status := ""
		if ok {
			status = turn.Status
		}
		c.mu.Unlock()
		if ok && (status == "done" || status == "error") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("turn did not finish")
		}
		time.Sleep(5 * time.Millisecond)
	}

	deadline = time.Now().Add(2 * time.Second)
	for {
		c.mu.Lock()
		_, ok := c.turns[id]
		c.mu.Unlock()
		if !ok {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("completed turn was not dropped")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

type staticProvider struct{ text string }

func staticReply(text string) staticProvider { return staticProvider{text: text} }

func (p staticProvider) Chat(context.Context, provider.ChatRequest) (provider.Message, error) {
	return provider.Message{Role: "assistant", Content: p.text}, nil
}
