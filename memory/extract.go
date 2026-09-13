package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/terracotta4u/golem/provider"
)

const extractPrompt = `Extract lasting facts and preferences about the user from this turn. Skip one-off task details, tool output, and anything that will not be useful in a later conversation.

Reply with a JSON array of strings and nothing else. Each string should be one durable memory, written in the third person. If there is nothing lasting to remember, reply with [].`

func Extract(ctx context.Context, p provider.Provider, turn []provider.Message) ([]string, error) {
	msgs := make([]provider.Message, 0, 1+len(turn))
	msgs = append(msgs, provider.Message{Role: "system", Content: extractPrompt})
	msgs = append(msgs, turn...)

	msg, err := p.Chat(ctx, provider.ChatRequest{Messages: msgs})
	if err != nil {
		return nil, fmt.Errorf("extract memories: %w", err)
	}
	return parseMemories(msg.Content), nil
}

func parseMemories(s string) []string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)

	var raw []string
	if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil
	}
	var out []string
	for _, m := range raw {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		out = append(out, m)
	}
	return out
}
