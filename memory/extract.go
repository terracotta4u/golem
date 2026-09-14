package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/terracotta4u/golem/provider"
)

const extractPrompt = `Extract lasting facts and preferences about the user from this turn. Skip one-off task details, tool output, and anything that will not be useful in a later conversation.

Each memory should be one durable fact, written in the third person. If there is nothing lasting to remember, return an empty list.`

const extractJSONPrompt = `Reply with a JSON array of strings and nothing else. If there is nothing lasting to remember, reply with [].`

func Extract(ctx context.Context, p provider.Provider, turn []provider.Message) ([]string, error) {
	// Use structured output when the provider supports it.
	if s, ok := p.(provider.Structured); ok {
		raw, err := s.ChatStructured(ctx, extractMessages(turn, false), memorySchema())
		if err != nil {
			return nil, fmt.Errorf("extract memories: %w", err)
		}
		got, ok := parseMemories(string(raw))
		if !ok {
			fmt.Fprintf(os.Stderr, "memory: extract: parse failed: %q\n", clip(string(raw), 200))
			return nil, nil
		}
		return got, nil
	}

	// Chat-only providers: ask for JSON in the prompt.
	msg, err := p.Chat(ctx, provider.ChatRequest{Messages: extractMessages(turn, true)})
	if err != nil {
		return nil, fmt.Errorf("extract memories: %w", err)
	}
	got, ok := parseMemories(msg.Content)
	if !ok {
		fmt.Fprintf(os.Stderr, "memory: extract: parse failed: %q\n", clip(msg.Content, 200))
		return nil, nil
	}
	return got, nil
}

func extractMessages(turn []provider.Message, jsonFallback bool) []provider.Message {
	prompt := extractPrompt
	if jsonFallback {
		prompt = extractPrompt + "\n\n" + extractJSONPrompt
	}
	msgs := make([]provider.Message, 0, 1+len(turn))
	msgs = append(msgs, provider.Message{Role: "system", Content: prompt})
	return append(msgs, turn...)
}

func memorySchema() provider.JSONSchema {
	return provider.JSONSchema{
		Name:   "memories",
		Strict: true,
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"memories": map[string]any{
					"type":  "array",
					"items": map[string]any{"type": "string"},
				},
			},
			"required":             []string{"memories"},
			"additionalProperties": false,
		},
	}
}

func parseMemories(s string) ([]string, bool) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, false
	}

	var raw []string
	if strings.HasPrefix(s, "{") {
		var obj struct {
			Memories *[]string `json:"memories"`
		}
		if err := json.Unmarshal([]byte(s), &obj); err != nil {
			return nil, false
		}
		if obj.Memories == nil {
			return nil, false
		}
		raw = *obj.Memories
	} else if err := json.Unmarshal([]byte(s), &raw); err != nil {
		return nil, false
	}
	return cleanMemories(raw), true
}

func cleanMemories(raw []string) []string {
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

func clip(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
