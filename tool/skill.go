package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/terracotta4u/golem/skill"
)

type Skill struct {
	skills []skill.Skill
	byName map[string]skill.Skill
}

func NewSkill(skills []skill.Skill) Skill {
	byName := make(map[string]skill.Skill, len(skills))
	for _, s := range skills {
		byName[s.Name] = s
	}
	return Skill{skills: skills, byName: byName}
}

func (s Skill) Skills() []skill.Skill {
	return s.skills
}

func (Skill) Spec() Spec {
	return Spec{
		Name:        "skill",
		Description: "Load a skill's instructions by name. Call this before following a skill listed in the system prompt. Returns the skill body, its directory, and any extra files.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type":        "string",
					"description": "Skill name from the system prompt catalog",
				},
			},
			"required": []string{"name"},
		},
	}
}

func (s Skill) Call(_ context.Context, args json.RawMessage) (string, error) {
	var input struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(args, &input); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if input.Name == "" {
		return "", fmt.Errorf("name is required")
	}

	sk, ok := s.byName[input.Name]
	if !ok {
		return "", fmt.Errorf("unknown skill: %s", input.Name)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "dir: %s\n", sk.Dir)
	if files := siblingFiles(sk.Dir); len(files) > 0 {
		b.WriteString("files:\n")
		for _, f := range files {
			fmt.Fprintf(&b, "- %s\n", f)
		}
	}
	b.WriteString("\n")
	b.WriteString(sk.Body)
	return b.String(), nil
}

func siblingFiles(dir string) []string {
	var files []string
	_ = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() == "SKILL.md" {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return nil
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	sort.Strings(files)
	return files
}
