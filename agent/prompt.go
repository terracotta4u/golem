package agent

import (
	"fmt"
	"os"
	"path/filepath"
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

const soulPrompt = `SOUL.md is where you store information about yourself.

SOUL.md:\n`

const userPrompt = `USER.md contains information about the user.

USER.md:\n`

func systemPrompt(dir string, tools ...tool.Tool) string {
	var b strings.Builder
	b.WriteString(basePrompt)
	if skills := skillList(tools); len(skills) > 0 {
		b.WriteString(skillsPrompt)
		for _, s := range skills {
			fmt.Fprintf(&b, "- %s: %s\n", s.Name, s.Description)
		}
	}
	if dir != "" {
		b.WriteString(soulPrompt)
		b.WriteString(workspaceFile(filepath.Join(dir, "SOUL.md")))
		b.WriteByte('\n')
		b.WriteString(userPrompt)
		b.WriteString(workspaceFile(filepath.Join(dir, "USER.md")))
		b.WriteByte('\n')
	}
	return strings.TrimSuffix(b.String(), "\n")
}

func workspaceFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Sprintf("(could not read: %v)", err)
	}
	return string(data)
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
