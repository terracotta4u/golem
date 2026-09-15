package remote

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/terracotta4u/golem/provider"
)

type Client struct {
	url   string
	token string
	http  *http.Client
}

func New(url, token string) *Client {
	return &Client{
		url:   strings.TrimRight(url, "/"),
		token: token,
		http:  http.DefaultClient,
	}
}

type Bound struct {
	client *Client
	model  string
}

func (c *Client) ForModel(model string) *Bound {
	return &Bound{client: c, model: model}
}

var (
	_ provider.Provider   = (*Bound)(nil)
	_ provider.Structured = (*Bound)(nil)
	_ provider.Embedder   = (*Bound)(nil)
)

type chatBody struct {
	Model    string             `json:"model"`
	Messages []provider.Message `json:"messages"`
	Tools    []toolBody         `json:"tools,omitempty"`
}

type toolBody struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type structuredBody struct {
	Model    string             `json:"model"`
	Messages []provider.Message `json:"messages"`
	Schema   schemaBody         `json:"schema"`
}

type schemaBody struct {
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
}

type embedBody struct {
	Model string   `json:"model"`
	Texts []string `json:"texts"`
}

type errorBody struct {
	Error *struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func (b *Bound) Chat(ctx context.Context, req provider.ChatRequest) (provider.Message, error) {
	status, raw, err := b.client.post(ctx, "/v1/chat", chatBody{
		Model:    b.model,
		Messages: req.Messages,
		Tools:    toTools(req.Tools),
	})
	if err != nil {
		return provider.Message{}, err
	}
	if err := responseError(status, raw); err != nil {
		return provider.Message{}, err
	}
	var msg provider.Message
	if err := json.Unmarshal(raw, &msg); err != nil {
		return provider.Message{}, fmt.Errorf("decode response: %w", err)
	}
	return msg, nil
}

func (b *Bound) ChatStructured(ctx context.Context, msgs []provider.Message, schema provider.JSONSchema) (json.RawMessage, error) {
	status, raw, err := b.client.post(ctx, "/v1/chat/structured", structuredBody{
		Model:    b.model,
		Messages: msgs,
		Schema: schemaBody{
			Name:   schema.Name,
			Strict: schema.Strict,
			Schema: schema.Schema,
		},
	})
	if err != nil {
		return nil, err
	}
	if err := responseError(status, raw); err != nil {
		return nil, err
	}
	var out struct {
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if len(out.Data) == 0 {
		return nil, fmt.Errorf("remote: empty structured response")
	}
	return out.Data, nil
}

func (b *Bound) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	status, raw, err := b.client.post(ctx, "/v1/embed", embedBody{
		Model: b.model,
		Texts: texts,
	})
	if err != nil {
		return nil, err
	}
	if err := responseError(status, raw); err != nil {
		return nil, err
	}
	var out struct {
		Vectors [][]float32 `json:"vectors"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if out.Vectors == nil {
		return nil, fmt.Errorf("remote: empty embedding response")
	}
	return out.Vectors, nil
}

func (c *Client) post(ctx context.Context, path string, payload any) (int, []byte, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, fmt.Errorf("marshal request: %w", err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(httpReq)
	if err != nil {
		return 0, nil, fmt.Errorf("remote request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read response: %w", err)
	}
	return resp.StatusCode, raw, nil
}

func responseError(status int, raw []byte) error {
	var parsed errorBody
	_ = json.Unmarshal(raw, &parsed)
	if parsed.Error != nil {
		msg := parsed.Error.Message
		if msg == "" {
			msg = string(raw)
		}
		if parsed.Error.Code == "unsupported_format" {
			return fmt.Errorf("%w: %s", provider.ErrUnsupportedFormat, msg)
		}
		return fmt.Errorf("remote: %s", msg)
	}
	if status != http.StatusOK {
		return fmt.Errorf("remote: unexpected status %d: %s", status, raw)
	}
	return nil
}

func toTools(defs []provider.ToolDef) []toolBody {
	if len(defs) == 0 {
		return nil
	}
	tools := make([]toolBody, 0, len(defs))
	for _, def := range defs {
		tools = append(tools, toolBody{
			Name:        def.Name,
			Description: def.Description,
			Parameters:  def.Parameters,
		})
	}
	return tools
}
