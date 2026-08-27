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

type Project struct {
	Name    string
	Version string
	Command string
	Dir     string
}

var nameRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type pyproject struct {
	Project struct {
		Name    string            `toml:"name"`
		Version string            `toml:"version"`
		Scripts map[string]string `toml:"scripts"`
	} `toml:"project"`
}

func Parse(data []byte) (Project, error) {
	var p pyproject
	if err := toml.Unmarshal(data, &p); err != nil {
		return Project{}, fmt.Errorf("parse %s: %w", FileName, err)
	}

	proj := Project{
		Name:    strings.TrimSpace(p.Project.Name),
		Version: strings.TrimSpace(p.Project.Version),
	}
	if proj.Name == "" {
		return Project{}, fmt.Errorf("missing name")
	}
	if len(proj.Name) > 64 || !nameRE.MatchString(proj.Name) {
		return Project{}, fmt.Errorf("invalid name %q", proj.Name)
	}
	if proj.Version == "" {
		return Project{}, fmt.Errorf("missing version")
	}
	command, err := scriptName(proj.Name, p.Project.Scripts)
	if err != nil {
		return Project{}, err
	}
	proj.Command = command
	return proj, nil
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

func Load(dir string) (Project, error) {
	path := filepath.Join(dir, FileName)
	data, err := os.ReadFile(path)
	if err != nil {
		return Project{}, err
	}
	p, err := Parse(data)
	if err != nil {
		return Project{}, fmt.Errorf("%s: %w", path, err)
	}
	return p, nil
}
