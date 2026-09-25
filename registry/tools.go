package registry

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sort"

	"github.com/terracotta4u/golem/tool"
)

const maxBody = 1 << 20

// Tools returns extension tools from live registrations.
func (r *Registry) Tools() []tool.Tool {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []tool.Tool
	for name, e := range r.exts {
		if !r.fresh(e) {
			r.dropLocked(name)
			continue
		}
		for _, tc := range e.tools {
			params, err := objectParams(tc.parameters)
			if err != nil {
				fmt.Fprintf(os.Stderr, "extension %s: tool %s: %v\n", name, tc.name, err)
				continue
			}
			out = append(out, extensionTool{
				name:        tc.name,
				description: tc.description,
				parameters:  params,
				callback:    e.callback,
				token:       r.token,
			})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Spec().Name < out[j].Spec().Name
	})
	return out
}

func objectParams(raw json.RawMessage) (map[string]any, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("tool parameters are required")
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil || obj == nil {
		return nil, fmt.Errorf("tool parameters must be an object")
	}
	return obj, nil
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
