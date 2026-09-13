package provider

import (
	"context"
	"strings"
	"testing"
)

func TestRegistryResolvesRegisteredEmbedder(t *testing.T) {
	reg := NewRegistry()
	var gotModel string
	stub := stubEmbedder{vecs: [][]float32{{0.1, 0.2}}}
	reg.RegisterEmbedder("stub", func(model string) (Embedder, error) {
		gotModel = model
		return stub, nil
	})

	got, err := reg.Embedder("stub", "text-embed-v1")
	if err != nil {
		t.Fatal(err)
	}
	if gotModel != "text-embed-v1" {
		t.Errorf("factory model = %q, want text-embed-v1", gotModel)
	}
	vecs, err := got.Embed(context.Background(), []string{"hello"})
	if err != nil {
		t.Fatal(err)
	}
	if len(vecs) != 1 || len(vecs[0]) != 2 || vecs[0][0] != 0.1 || vecs[0][1] != 0.2 {
		t.Errorf("vecs = %v", vecs)
	}
}

func TestRegistryUnknownProvider(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterEmbedder("stub", func(string) (Embedder, error) {
		return stubEmbedder{}, nil
	})

	_, err := reg.Embedder("openrouter", "openai/text-embedding-3-small")
	if err == nil {
		t.Fatal("want error for unknown provider")
	}
	if !strings.Contains(err.Error(), "openrouter") {
		t.Errorf("err = %v, want unknown provider named", err)
	}
}

func TestRegistryFactoryError(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterEmbedder("stub", func(string) (Embedder, error) {
		return nil, errString("no key")
	})
	_, err := reg.Embedder("stub", "model")
	if err == nil || !strings.Contains(err.Error(), "no key") {
		t.Fatalf("err = %v, want no key", err)
	}
}

type stubEmbedder struct {
	vecs [][]float32
}

func (s stubEmbedder) Embed(_ context.Context, _ []string) ([][]float32, error) {
	return s.vecs, nil
}

type errString string

func (e errString) Error() string { return string(e) }
