package web

import (
	"testing"

	"github.com/terracotta4u/golem/provider"
)

func TestChatItems(t *testing.T) {
	echo := provider.ToolCall{
		ID:   "call_1",
		Type: "function",
		Function: provider.FunctionCall{
			Name:      "echo",
			Arguments: `{"text":"hi"}`,
		},
	}
	read := provider.ToolCall{
		ID:   "call_2",
		Type: "function",
		Function: provider.FunctionCall{
			Name:      "read",
			Arguments: `{"path":"a"}`,
		},
	}

	t.Run("pairs tools with following reply", func(t *testing.T) {
		got := chatItems([]provider.Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", ToolCalls: []provider.ToolCall{echo}},
			{Role: "tool", ToolCallID: "call_1", Content: "pong"},
			{Role: "assistant", Content: "all set"},
		})
		if len(got) != 2 {
			t.Fatalf("items = %d, want 2", len(got))
		}
		if got[0].Role != "user" || got[0].Content != "hello" {
			t.Errorf("item 0 = %+v", got[0])
		}
		if got[1].Role != "assistant" || got[1].Content != "all set" || len(got[1].Tools) != 1 {
			t.Fatalf("item 1 = %+v", got[1])
		}
		if g := got[1].Tools[0]; g.Name != "echo" || g.Args != `{"text":"hi"}` || g.Result != "pong" {
			t.Errorf("tool = %+v", g)
		}
	})

	t.Run("accumulates rounds onto the reply", func(t *testing.T) {
		got := chatItems([]provider.Message{
			{Role: "assistant", ToolCalls: []provider.ToolCall{echo}},
			{Role: "tool", ToolCallID: "call_1", Content: "pong"},
			{Role: "assistant", ToolCalls: []provider.ToolCall{read}},
			{Role: "tool", ToolCallID: "call_2", Content: "file"},
			{Role: "assistant", Content: "done"},
		})
		if len(got) != 1 || got[0].Content != "done" || len(got[0].Tools) != 2 {
			t.Fatalf("items = %+v", got)
		}
		if got[0].Tools[0].Name != "echo" || got[0].Tools[1].Name != "read" {
			t.Errorf("tools = %+v", got[0].Tools)
		}
	})

	t.Run("keeps tools when there is no reply", func(t *testing.T) {
		got := chatItems([]provider.Message{
			{Role: "assistant", ToolCalls: []provider.ToolCall{echo}},
			{Role: "tool", ToolCallID: "call_1", Content: "pong"},
		})
		if len(got) != 1 || got[0].Role != "assistant" || got[0].Content != "" || len(got[0].Tools) != 1 {
			t.Fatalf("items = %+v", got)
		}
	})

	t.Run("skips orphan tool messages", func(t *testing.T) {
		got := chatItems([]provider.Message{
			{Role: "tool", ToolCallID: "call_1", Content: "pong"},
			{Role: "assistant", Content: "hi"},
		})
		if len(got) != 1 || got[0].Content != "hi" || len(got[0].Tools) != 0 {
			t.Fatalf("items = %+v", got)
		}
	})
}
