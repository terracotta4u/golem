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
	Dir         string
	Provider    Decl
	Channel     Decl
	Tools       string
}

var nameRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

type golemDecl struct {
	ID         string `toml:"id"`
	Entrypoint string `toml:"entrypoint"`
}

type pyproject struct {
	Project struct {
		Name        string `toml:"name"`
		Version     string `toml:"version"`
		Description string `toml:"description"`
	} `toml:"project"`
	Tool struct {
		Golem struct {
			Provider *golemDecl `toml:"provider"`
			Channel  *golemDecl `toml:"channel"`
			Tools    string     `toml:"tools"`
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
	provider, err := decl(p.Tool.Golem.Provider, "provider")
	if err != nil {
		return Project{}, err
	}
	channel, err := decl(p.Tool.Golem.Channel, "channel")
	if err != nil {
		return Project{}, err
	}
	tools := strings.TrimSpace(p.Tool.Golem.Tools)
	if tools != "" {
		if err := entrypoint(tools); err != nil {
			return Project{}, err
		}
	}
	if provider.ID == "" && channel.ID == "" && tools == "" {
		return Project{}, fmt.Errorf("declare a provider, channel, or tools in [tool.golem]")
	}
	proj.Provider = provider
	proj.Channel = channel
	proj.Tools = tools
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
	if err := entrypoint(entry); err != nil {
		return Decl{}, err
	}
	return Decl{ID: id, Entrypoint: entry}, nil
}

func entrypoint(entry string) error {
	module, attr, ok := strings.Cut(entry, ":")
	if !ok || module == "" || attr == "" || strings.Contains(attr, ":") {
		return fmt.Errorf("invalid entrypoint %q", entry)
	}
	return nil
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
