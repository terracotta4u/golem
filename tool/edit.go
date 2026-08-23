package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type Edit struct{}

func NewEdit() Edit {
	return Edit{}
}

func (Edit) Spec() Spec {
	return Spec{
		Name:        "edit",
		Description: "Replace exact text in an existing file. old_string must match uniquely unless replace_all is true. Prefer this over rewriting the whole file when changing a small section.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"path": map[string]any{
					"type":        "string",
					"description": "Path to the file to edit",
				},
				"old_string": map[string]any{
					"type":        "string",
					"description": "Exact text to find",
				},
				"new_string": map[string]any{
					"type":        "string",
					"description": "Replacement text",
				},
				"replace_all": map[string]any{
					"type":        "boolean",
					"description": "Replace every occurrence of old_string instead of requiring a unique match",
				},
			},
			"required": []string{"path", "old_string", "new_string"},
		},
	}
}

func (Edit) Call(_ context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Path       string `json:"path"`
		OldString  string `json:"old_string"`
		NewString  string `json:"new_string"`
		ReplaceAll bool   `json:"replace_all"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if input.Path == "" {
		return "", fmt.Errorf("path is required")
	}
	if input.OldString == "" {
		return "", fmt.Errorf("old_string is required")
	}
	if input.OldString == input.NewString {
		return "", fmt.Errorf("old_string and new_string are identical")
	}

	data, err := os.ReadFile(input.Path)
	if err != nil {
		return "", err
	}

	content := string(data)
	count := strings.Count(content, input.OldString)
	if count == 0 {
		return "", fmt.Errorf("old_string not found in %s", input.Path)
	}
	if count > 1 && !input.ReplaceAll {
		return "", fmt.Errorf("old_string found %d times in %s; provide more context or set replace_all", count, input.Path)
	}

	updated := strings.Replace(content, input.OldString, input.NewString, 1)
	if input.ReplaceAll {
		updated = strings.ReplaceAll(content, input.OldString, input.NewString)
	}
	if err := os.WriteFile(input.Path, []byte(updated), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("updated %s (%d replacement(s))", input.Path, count), nil
}
