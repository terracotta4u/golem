package tool

import "github.com/terracotta4u/golem/skill"

// Builtins returns the tools the agent exposes.
// The skill tool is included when no skills are loaded.
func Builtins(skills []skill.Skill) []Tool {
	return []Tool{
		NewRead(),
		NewWrite(),
		NewEdit(),
		NewShell(),
		NewSkill(skills),
	}
}

// Reserved reports whether name belongs to a builtin tool.
func Reserved(name string) bool {
	for _, t := range Builtins(nil) {
		if t.Spec().Name == name {
			return true
		}
	}
	return false
}
