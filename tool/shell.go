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

// TODO: make these configurable
const (
	shellTimeout   = 30 * time.Second
	maxShellOutput = 32 * 1024
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
			},
			"required": []string{"command"},
		},
	}
}

func (Shell) Call(ctx context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if input.Command == "" {
		return "", fmt.Errorf("command is required")
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, shellTimeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, "bash", "-c", input.Command)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	output := truncate(out.String(), maxShellOutput)
	if output == "" {
		output = "(no output)"
	}

	if err != nil {
		if ctx.Err() != nil {
			return fmt.Sprintf("timed out after %s\n%s", shellTimeout, output), nil
		}
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Sprintf("exit %d\n%s", exitErr.ExitCode(), output), nil
		}
		return "", err
	}

	return output, nil
}
