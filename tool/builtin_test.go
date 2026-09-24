package tool

import (
	"testing"

	"github.com/terracotta4u/golem/skill"
)

func TestBuiltins(t *testing.T) {
	got := Builtins(nil)
	if len(got) != 5 {
		t.Fatalf("len = %d, want 5", len(got))
	}
	for _, name := range []string{"read", "write", "edit", "shell", "skill"} {
		if !hasName(got, name) {
			t.Fatalf("missing %s", name)
		}
	}

	loaded := Builtins([]skill.Skill{{Name: "commit"}})
	sk, ok := loaded[4].(Skill)
	if !ok || len(sk.Skills()) != 1 || sk.Skills()[0].Name != "commit" {
		t.Fatal("skill tool missing the loaded skill")
	}
}

func TestReserved(t *testing.T) {
	for _, name := range []string{"read", "write", "edit", "shell", "skill"} {
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
