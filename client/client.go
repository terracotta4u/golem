package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	url   string
	token string
	http  *http.Client
}

type postTurnRequest struct {
	Channel string `json:"channel"`
	Text    string `json:"text"`
}

type postTurnResponse struct {
	ID    string `json:"id"`
	Error string `json:"error,omitempty"`
}

type Turn struct {
	ID     string   `json:"id"`
	Status string   `json:"status"`
	Text   string   `json:"text,omitempty"`
	Error  string   `json:"error,omitempty"`
	Log    []string `json:"log,omitempty"`
}

func New(url, token string) *Client {
	return &Client{
		url:   url,
		token: token,
		http:  &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Health() error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_, err := c.do(ctx, http.MethodGet, c.url+"/v1/health", nil)
	return err
}

func (c *Client) Send(ctx context.Context, channel, convID, text string, onLog func(string)) (string, error) {
	body, err := json.Marshal(postTurnRequest{
		Channel: channel,
		Text:    text,
	})
	if err != nil {
		return "", err
	}

	raw, err := c.do(ctx, http.MethodPost, c.url+"/v1/conversations/"+convID+"/turns", body)
	if err != nil {
		return "", err
	}
	var accepted postTurnResponse
	if err := json.Unmarshal(raw, &accepted); err != nil {
		return "", fmt.Errorf("decode accept: %w", err)
	}
	if accepted.Error != "" {
		return "", fmt.Errorf("golem: %s", accepted.Error)
	}
	if accepted.ID == "" {
		return "", fmt.Errorf("golem: missing turn id")
	}
	return c.wait(ctx, accepted.ID, onLog)
}

func (c *Client) wait(ctx context.Context, id string, onLog func(string)) (string, error) {
	ticker := time.NewTicker(400 * time.Millisecond)
	defer ticker.Stop()
	seen := 0

	for {
		raw, err := c.do(ctx, http.MethodGet, c.url+"/v1/turns/"+id, nil)
		if err != nil {
			return "", err
		}
		var t Turn
		if err := json.Unmarshal(raw, &t); err != nil {
			return "", fmt.Errorf("decode turn: %w", err)
		}
		if onLog != nil {
			for ; seen < len(t.Log); seen++ {
				onLog(t.Log[seen])
			}
		}
		switch t.Status {
		case "done":
			return t.Text, nil
		case "error":
			if t.Error == "" {
				return "", fmt.Errorf("golem: turn failed")
			}
			return "", fmt.Errorf("%s", t.Error)
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
}

func (c *Client) do(ctx context.Context, method, url string, body []byte) ([]byte, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return nil, err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("golem request: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read golem response: %w", err)
	}
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("golem: unexpected status %d: %s", resp.StatusCode, raw)
	}
	return raw, nil
}
