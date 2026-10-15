package tool

import (
	"testing"
)

func TestBuiltins(t *testing.T) {
	got := Builtins()
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
	for _, name := range []string{"read", "write", "edit", "shell"} {
		if !hasName(got, name) {
			t.Fatalf("missing %s", name)
		}
	}
}

func TestReserved(t *testing.T) {
	for _, name := range []string{"read", "write", "edit", "shell"} {
		if !Reserved(name) {
			t.Fatalf("%s should be reserved", name)
		}
	}
	if Reserved("weather") {
		t.Fatal("weather should be free")
	}
}

func hasName(tools []Tool, name string) bool {
	for _, t := range tools {
		if t.Spec().Name == name {
			return true
		}
	}
	return false
}
