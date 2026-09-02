package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"
)

const (
	defaultShellTimeout   = 30 * time.Second
	defaultMaxShellOutput = 32 * 1024
)

type Shell struct{}

func NewShell() Shell {
	return Shell{}
}

func (Shell) Spec() Spec {
	return Spec{
		Name:        "shell",
		Description: "Run a bash command in the current working directory and return its output.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"command": map[string]any{
					"type":        "string",
					"description": "The bash command to execute",
				},
				"timeout": map[string]any{
					"type":        "integer",
					"description": "Timeout in seconds. Defaults to 30.",
				},
				"max_output": map[string]any{
					"type":        "integer",
					"description": "Maximum number of output bytes to return. Defaults to 32768.",
				},
			},
			"required": []string{"command"},
		},
	}
}

func (Shell) Call(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Command   string `json:"command"`
		Timeout   int    `json:"timeout"`
		MaxOutput int    `json:"max_output"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if input.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	timeout := defaultShellTimeout
	if input.Timeout > 0 {
		timeout = time.Duration(input.Timeout) * time.Second
	}
	maxOutput := defaultMaxShellOutput
	if input.MaxOutput > 0 {
		maxOutput = input.MaxOutput
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", input.Command)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	output := truncate(out.String(), maxOutput)
	if output == "" {
		output = "(no output)"
	}

	if err != nil {
		if ctx.Err() != nil {
			return fmt.Sprintf("timed out after %s\n%s", timeout, output), nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Sprintf("exit %d\n%s", exitErr.ExitCode(), output), nil
		}
		return "", err
	}

	return output, nil
}
