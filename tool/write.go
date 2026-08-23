package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Write struct{}

func NewWrite() Write {
	return Write{}
}

func (Write) Spec() Spec {
	return Spec{
		Name:        "write",
		Description: "Create or overwrite a file with the given contents. Prefer this over shell redirection. Creates parent directories if needed.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to write",
				},
				"content": map[string]any{
					"type":        "string",
					"description": "Full contents to write to the file",
				},
			},
			"required": []string{"path", "content"},
		},
	}
}

func (Write) Call(_ context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if input.Path == "" {
		return "", fmt.Errorf("path is required")
	}

	dir := filepath.Dir(input.Path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return "", err
		}
	}

	if err := os.WriteFile(input.Path, []byte(input.Content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %s (%d bytes)", input.Path, len(input.Content)), nil
}
