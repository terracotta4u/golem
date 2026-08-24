package agent

import (
	"fmt"
	"strings"

	"github.com/terracotta4u/golem/skill"
	"github.com/terracotta4u/golem/tool"
)

const basePrompt = `You are Golem, a personal AI assistant. 

At your disposal you have four core tools to complete tasks:
1. Read - Read the contents of a file.
2. Write - Create a new file and write to it.
3. Edit - Update the contents of a file.
4. Shell - Execute a shell command.\n\n`

const skillsPrompt = `When a listed skill applies, load it with the skill tool before following it. 

Skills:\n`

func systemPrompt(tools ...tool.Tool) string {
	skills := skillList(tools)
	if len(skills) == 0 {
		return basePrompt
	}
	var b strings.Builder
	b.WriteString(basePrompt)
	b.WriteString(skillsPrompt)
	for _, s := range skills {
		fmt.Fprintf(&b, "- %s: %s\n", s.Name, s.Description)
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func skillList(tools []tool.Tool) []skill.Skill {
	for _, t := range tools {
		s, ok := t.(tool.Skill)
		if !ok {
			continue
		}
		return s.Skills()
	}
	return nil
}
