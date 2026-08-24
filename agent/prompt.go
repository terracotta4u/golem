package agent

import (
	"strings"

	"github.com/terracotta4u/golem/tool"
)

const basePrompt = `You are Golem, a personal AI assistant. 

At your disposal you have four core tools to complete tasks:
1. Read - Read the contents of a file.
2. Write - Create a new file and write to it.
3. Edit - Update the contents of a file.
4. Shell - Execute a shell command.`

type extraPrompt interface {
	ExtraPrompt() string
}

func systemPrompt(tools ...tool.Tool) string {
	var b strings.Builder
	b.WriteString(basePrompt)
	for _, t := range tools {
		p, ok := t.(extraPrompt)
		if !ok {
			continue
		}
		extra := strings.TrimSpace(p.ExtraPrompt())
		if extra == "" {
			continue
		}
		b.WriteString("\n\n")
		b.WriteString(extra)
	}
	return b.String()
}
