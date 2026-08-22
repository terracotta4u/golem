package openrouter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/terracotta4u/golem/provider"
)

const (
	defaultURL = "https://openrouter.ai/api/v1/chat/completions"
	referer    = "https://github.com/terracotta4u/golem"
	title      = "golem"
)

type Client struct {
	apiKey string
	model  string
	url    string
	http   *http.Client
}

func New(apiKey, model string) *Client {
	return &Client{
		apiKey: apiKey,
		model:  model,
		url:    defaultURL,
		http:   http.DefaultClient,
	}
}

type chatRequest struct {
	Model    string             `json:"model"`
	Messages []provider.Message `json:"messages"`
	Tools    []chatTool         `json:"tools,omitempty"`
}

type chatTool struct {
	Type     string           `json:"type"`
	Function chatToolFunction `json:"function"`
}

type chatToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type chatResponse struct {
	Choices []struct {
		Message provider.Message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) Chat(ctx context.Context, req provider.ChatRequest) (provider.Message, error) {
	body, err := json.Marshal(chatRequest{
		Model:    c.model,
		Messages: req.Messages,
		Tools:    toTools(req.Tools),
	})
	if err != nil {
		return provider.Message{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return provider.Message{}, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("HTTP-Referer", referer)
	httpReq.Header.Set("X-OpenRouter-Title", title)

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return provider.Message{}, fmt.Errorf("openrouter request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return provider.Message{}, fmt.Errorf("read response: %w", err)
	}

	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return provider.Message{}, fmt.Errorf("decode response: %w", err)
	}
	if parsed.Error != nil && parsed.Error.Message != "" {
		return provider.Message{}, fmt.Errorf("openrouter: %s", parsed.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return provider.Message{}, fmt.Errorf("openrouter: unexpected status %d: %s", resp.StatusCode, raw)
	}
	if len(parsed.Choices) == 0 {
		return provider.Message{}, fmt.Errorf("openrouter: empty response")
	}

	return parsed.Choices[0].Message, nil
}

func toTools(defs []provider.ToolDef) []chatTool {
	if len(defs) == 0 {
		return nil
	}
	tools := make([]chatTool, 0, len(defs))
	for _, def := range defs {
		tools = append(tools, chatTool{
			Type: "function",
			Function: chatToolFunction{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  def.Parameters,
			},
		})
	}
	return tools
}
