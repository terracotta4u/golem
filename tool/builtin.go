package tool

import "github.com/terracotta4u/golem/skill"

// owned is every tool Golem reserves. The name comes from Spec.
// skill stays here so the name stays reserved when no skills are loaded.
func owned() []Tool {
	return []Tool{
		NewRead(),
		NewWrite(),
		NewEdit(),
		NewShell(),
		NewSkill(nil),
	}
}

// Builtins returns the tools the agent exposes.
// With no skills loaded, the skill tool is omitted.
func Builtins(skills []skill.Skill) []Tool {
	out := make([]Tool, 0, 5)
	for _, t := range owned() {
		if _, ok := t.(Skill); ok {
			if len(skills) == 0 {
				continue
			}
			out = append(out, NewSkill(skills))
			continue
		}
		out = append(out, t)
	}
	return out
}

// Reserved reports whether name belongs to a builtin tool.
func Reserved(name string) bool {
	for _, t := range owned() {
		if t.Spec().Name == name {
			return true
		}
	}
	return false
}
