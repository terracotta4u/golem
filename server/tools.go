package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/terracotta4u/golem/tool"
)

// Tools returns extension tools from live registrations.
func (s *Server) Tools() []tool.Tool {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []tool.Tool
	for name, e := range s.exts {
		if !s.fresh(e) {
			s.dropLocked(name)
			continue
		}
		for _, cap := range e.caps {
			if cap.Kind != "tool" {
				continue
			}
			params := map[string]any{}
			if err := json.Unmarshal(cap.Parameters, &params); err != nil {
				continue
			}
			out = append(out, extensionTool{
				name:        cap.Name,
				description: cap.Description,
				parameters:  params,
				callback:    e.callback,
				token:       s.opts.Token,
			})
		}
	}
	return out
}

type extensionTool struct {
	name        string
	description string
	parameters  map[string]any
	callback    string
	token       string
}

func (t extensionTool) Spec() tool.Spec {
	return tool.Spec{
		Name:        t.name,
		Description: t.description,
		Parameters:  t.parameters,
	}
}

func (t extensionTool) Call(ctx context.Context, args json.RawMessage) (string, error) {
	if len(args) == 0 {
		args = []byte("{}")
	}
	if !json.Valid(args) {
		return "", fmt.Errorf("invalid arguments")
	}
	endpoint := t.callback + "/v1/tools/" + url.PathEscape(t.name)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(args))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if t.token != "" {
		req.Header.Set("Authorization", "Bearer "+t.token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return "", err
	}
	var body struct {
		Result string `json:"result"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		return "", fmt.Errorf("tool %s: status %d", t.name, resp.StatusCode)
	}
	if body.Error != nil {
		msg := body.Error.Message
		if msg == "" {
			msg = fmt.Sprintf("status %d", resp.StatusCode)
		}
		return "", fmt.Errorf("%s", msg)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tool %s: status %d", t.name, resp.StatusCode)
	}
	return body.Result, nil
}
