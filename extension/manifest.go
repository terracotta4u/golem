package extension

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const FileName = "pyproject.toml"

type Manifest struct {
	Name        string
	Version     string
	Description string
	Command     string
	Dir         string
}

var nameRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type pyproject struct {
	Project struct {
		Name        string            `toml:"name"`
		Version     string            `toml:"version"`
		Description string            `toml:"description"`
		Scripts     map[string]string `toml:"scripts"`
	} `toml:"project"`
}

func Parse(data []byte) (Manifest, error) {
	var p pyproject
	if err := toml.Unmarshal(data, &p); err != nil {
		return Manifest{}, fmt.Errorf("parse %s: %w", FileName, err)
	}

	m := Manifest{
		Name:        strings.TrimSpace(p.Project.Name),
		Version:     strings.TrimSpace(p.Project.Version),
		Description: strings.TrimSpace(p.Project.Description),
	}
	if m.Name == "" {
		return Manifest{}, fmt.Errorf("missing name")
	}
	if len(m.Name) > 64 || !nameRE.MatchString(m.Name) {
		return Manifest{}, fmt.Errorf("invalid name %q", m.Name)
	}
	if m.Version == "" {
		return Manifest{}, fmt.Errorf("missing version")
	}
	command, err := scriptName(m.Name, p.Project.Scripts)
	if err != nil {
		return Manifest{}, err
	}
	m.Command = command
	return m, nil
}

func scriptName(project string, scripts map[string]string) (string, error) {
	if len(scripts) == 0 {
		return "", fmt.Errorf("missing [project.scripts]")
	}
	if _, ok := scripts[project]; ok {
		return project, nil
	}
	if len(scripts) == 1 {
		for name := range scripts {
			if strings.TrimSpace(name) == "" {
				return "", fmt.Errorf("missing [project.scripts]")
			}
			return name, nil
		}
	}
	return "", fmt.Errorf("no [project.scripts] entry for %q", project)
}

func Load(dir string) (Manifest, error) {
	path := filepath.Join(dir, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	m, err := Parse(data)
	if err != nil {
		return Manifest{}, fmt.Errorf("%s: %w", path, err)
	}
	return m, nil
}
