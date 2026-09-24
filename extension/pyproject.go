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

// Decl is a provider or channel named in [tool.golem].
type Decl struct {
	ID         string
	Entrypoint string
}

type Project struct {
	Name        string
	Version     string
	Description string
	Command     string
	Dir         string
	Provider    Decl
	Channel     Decl
}

var nameRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type golemDecl struct {
	ID         string `toml:"id"`
	Entrypoint string `toml:"entrypoint"`
}

type pyproject struct {
	Project struct {
		Name        string            `toml:"name"`
		Version     string            `toml:"version"`
		Description string            `toml:"description"`
		Scripts     map[string]string `toml:"scripts"`
	} `toml:"project"`
	Tool struct {
		Golem struct {
			Provider *golemDecl `toml:"provider"`
			Channel  *golemDecl `toml:"channel"`
		} `toml:"golem"`
	} `toml:"tool"`
}

func Parse(data []byte) (Project, error) {
	var p pyproject
	if err := toml.Unmarshal(data, &p); err != nil {
		return Project{}, fmt.Errorf("parse %s: %w", FileName, err)
	}

	proj := Project{
		Name:        strings.TrimSpace(p.Project.Name),
		Version:     strings.TrimSpace(p.Project.Version),
		Description: strings.TrimSpace(p.Project.Description),
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
	provider, err := decl(p.Tool.Golem.Provider, "provider")
	if err != nil {
		return Project{}, err
	}
	channel, err := decl(p.Tool.Golem.Channel, "channel")
	if err != nil {
		return Project{}, err
	}
	proj.Provider = provider
	proj.Channel = channel
	return proj, nil
}

func decl(raw *golemDecl, kind string) (Decl, error) {
	if raw == nil {
		return Decl{}, nil
	}
	id := strings.TrimSpace(raw.ID)
	if id == "" {
		return Decl{}, fmt.Errorf("%s id is required", kind)
	}
	entry := strings.TrimSpace(raw.Entrypoint)
	if entry == "" {
		return Decl{}, fmt.Errorf("%s entrypoint is required", kind)
	}
	module, attr, ok := strings.Cut(entry, ":")
	if !ok || module == "" || attr == "" || strings.Contains(attr, ":") {
		return Decl{}, fmt.Errorf("invalid entrypoint %q", entry)
	}
	return Decl{ID: id, Entrypoint: entry}, nil
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
