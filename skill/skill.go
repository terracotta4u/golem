package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Skill struct {
	Name        string
	Description string
	Body        string
	Dir         string
}

var nameRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func Parse(data []byte) (Skill, error) {
	s := strings.ReplaceAll(string(data), "\r\n", "\n")
	if !strings.HasPrefix(s, "---\n") {
		return Skill{}, fmt.Errorf("missing YAML frontmatter")
	}
	rest := strings.TrimPrefix(s, "---\n")
	i := strings.Index(rest, "\n---")
	if i < 0 {
		return Skill{}, fmt.Errorf("unterminated YAML frontmatter")
	}

	var meta struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(rest[:i]), &meta); err != nil {
		return Skill{}, fmt.Errorf("frontmatter: %w", err)
	}

	name := strings.TrimSpace(meta.Name)
	if name == "" {
		return Skill{}, fmt.Errorf("missing name")
	}
	if len(name) > 64 || !nameRE.MatchString(name) {
		return Skill{}, fmt.Errorf("invalid name %q", name)
	}

	desc := strings.TrimSpace(meta.Description)
	if desc == "" {
		return Skill{}, fmt.Errorf("missing description")
	}

	body := strings.TrimSpace(strings.TrimPrefix(rest[i+len("\n---"):], "\n"))
	return Skill{Name: name, Description: desc, Body: body}, nil
}

func LoadDir(root string) ([]Skill, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}

	var skills []Skill
	seen := make(map[string]string)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "skill %s: %v\n", e.Name(), err)
			continue
		}
		sk, err := Parse(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skill %s: %v\n", e.Name(), err)
			continue
		}
		if prev, ok := seen[sk.Name]; ok {
			return nil, fmt.Errorf("duplicate skill name %q in %s and %s", sk.Name, prev, dir)
		}
		seen[sk.Name] = dir
		sk.Dir = dir
		skills = append(skills, sk)
	}
	sort.Slice(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
	return skills, nil
}
