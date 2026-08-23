package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// TODO: make these configurable
const maxReadOutput = 32 * 1024

type Read struct{}

func NewRead() Read {
	return Read{}
}

func (Read) Spec() Spec {
	return Spec{
		Name:        "read",
		Description: "Read the contents of a file. Prefer this over shell commands like cat. Lines are numbered to help with edits. Use offset and limit for large files.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to read",
				},
				"offset": map[string]any{
					"type":        "integer",
					"description": "1-indexed line number to start reading from",
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of lines to return",
				},
			},
			"required": []string{"path"},
		},
	}
}

func (Read) Call(_ context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Path   string `json:"path"`
		Offset int    `json:"offset"`
		Limit  int    `json:"limit"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if input.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	data, err := os.ReadFile(input.Path)
	if err != nil {
		return "", err
	}
	if len(data) == 0 {
		return "(empty file)", nil
	}

	lines := strings.Split(string(data), "\n")
	start := 1
	if input.Offset > 0 {
		start = input.Offset
	}
	if start > len(lines) {
		return "", fmt.Errorf("offset %d is past end of file (%d lines)", start, len(lines))
	}

	end := len(lines)
	if input.Limit > 0 && start-1+input.Limit < end {
		end = start - 1 + input.Limit
	}

	var b strings.Builder
	for i, line := range lines[start-1 : end] {
		fmt.Fprintf(&b, "%6d|%s\n", start+i, line)
	}
	if b.Len() == 0 {
		return "(empty file)", nil
	}
	return truncate(b.String(), maxReadOutput), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "\n... (truncated)"
}
