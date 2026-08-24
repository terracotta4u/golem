package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/terracotta4u/golem/provider"
	"github.com/terracotta4u/golem/store"
	"github.com/terracotta4u/golem/tool"
)

// TODO: make this configurable
const maxToolRounds = 20

type Agent struct {
	provider provider.Provider
	tools    map[string]tool.Tool
	defs     []provider.ToolDef
}

func New(p provider.Provider, tools ...tool.Tool) *Agent {
	byName := make(map[string]tool.Tool, len(tools))
	defs := make([]provider.ToolDef, 0, len(tools))
	for _, t := range tools {
		spec := t.Spec()
		byName[spec.Name] = t
		defs = append(defs, provider.ToolDef{
			Name:        spec.Name,
			Description: spec.Description,
			Parameters:  spec.Parameters,
		})
	}
	return &Agent{provider: p, tools: byName, defs: defs}
}

type Session struct {
	agent  *Agent
	store  store.Store
	conv   store.Conversation
	OnTool func(name, args string)
}

func (a *Agent) Session(st store.Store, conv store.Conversation) *Session {
	return &Session{agent: a, store: st, conv: conv}
}

func (s *Session) ID() string { return s.conv.ID }

func (s *Session) Send(ctx context.Context, input string) (string, error) {
	s.conv.SetTitleFrom(input)
	s.conv.Messages = append(s.conv.Messages, provider.Message{Role: "user", Content: input})

	for range maxToolRounds {
		msg, err := s.agent.provider.Chat(ctx, provider.ChatRequest{
			Messages: withSystemPrompt(s.conv.Messages),
			Tools:    s.agent.defs,
		})
		if err != nil {
			if n := len(s.conv.Messages); n > 0 && s.conv.Messages[n-1].Role == "user" {
				s.conv.Messages = s.conv.Messages[:n-1]
			}
			return "", err
		}

		s.conv.Messages = append(s.conv.Messages, msg)
		if len(msg.ToolCalls) == 0 {
			if err := s.persist(); err != nil {
				return msg.Content, err
			}
			return msg.Content, nil
		}

		for _, call := range msg.ToolCalls {
			if s.OnTool != nil {
				s.OnTool(call.Function.Name, call.Function.Arguments)
			}
			s.conv.Messages = append(s.conv.Messages, provider.Message{
				Role:       "tool",
				ToolCallID: call.ID,
				Content:    s.agent.runTool(ctx, call),
			})
		}
	}

	return "", fmt.Errorf("exceeded %d tool rounds", maxToolRounds)
}

func withSystemPrompt(msgs []provider.Message) []provider.Message {
	out := make([]provider.Message, 0, 1+len(msgs))
	out = append(out, provider.Message{Role: "system", Content: systemPrompt})
	return append(out, msgs...)
}

func (s *Session) persist() error {
	s.conv.UpdatedAt = time.Now().UTC()
	return s.store.Save(s.conv)
}

func (a *Agent) runTool(ctx context.Context, call provider.ToolCall) string {
	t, ok := a.tools[call.Function.Name]
	if !ok {
		return fmt.Sprintf("unknown tool: %s", call.Function.Name)
	}

	result, err := t.Call(ctx, json.RawMessage(call.Function.Arguments))
	if err != nil {
		return fmt.Sprintf("error: %v", err)
	}
	return result
}
