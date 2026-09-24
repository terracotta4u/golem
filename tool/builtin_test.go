package tool

import (
	"testing"

	"github.com/terracotta4u/golem/skill"
)

func TestBuiltinsOmitSkillWhenEmpty(t *testing.T) {
	got := Builtins(nil)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4", len(got))
	}
	for _, name := range []string{"read", "write", "edit", "shell"} {
		if !hasName(got, name) {
			t.Fatalf("missing %s", name)
		}
	}
	if hasName(got, "skill") {
		t.Fatal("skill tool included with no skills")
	}
	if !Reserved("skill") {
		t.Fatal("skill name should stay reserved")
	}
}

func TestBuiltinsIncludeLoadedSkill(t *testing.T) {
	got := Builtins([]skill.Skill{{Name: "commit"}})
	if len(got) != 5 || !hasName(got, "skill") {
		t.Fatalf("tools = %v", names(got))
	}
	sk, ok := got[4].(Skill)
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

func names(tools []Tool) []string {
	out := make([]string, len(tools))
	for i, t := range tools {
		out[i] = t.Spec().Name
	}
	return out
}
