package tool

import (
	"context"
	"encoding/json"
)

type Spec struct {
	Name        string
	Description string
	Parameters  map[string]any
}

type Tool interface {
	Spec() Spec
	Call(ctx context.Context, args json.RawMessage) (string, error)
}
