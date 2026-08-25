package extension

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const FileName = "golem.json"

type Manifest struct {
	Name        string   `json:"name"`
	Kind        string   `json:"kind"`
	Description string   `json:"description,omitempty"`
	Command     string   `json:"command"`
	Args        []string `json:"args,omitempty"`
	Env         []string `json:"env,omitempty"`
}

var nameRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func Parse(data []byte) (Manifest, error) {
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse %s: %w", FileName, err)
	}

	m.Name = strings.TrimSpace(m.Name)
	if m.Name == "" {
		return Manifest{}, fmt.Errorf("missing name")
	}
	if len(m.Name) > 64 || !nameRE.MatchString(m.Name) {
		return Manifest{}, fmt.Errorf("invalid name %q", m.Name)
	}

	m.Kind = strings.TrimSpace(m.Kind)
	if m.Kind == "" {
		return Manifest{}, fmt.Errorf("missing kind")
	}
	if m.Kind == "provider" {
		return Manifest{}, fmt.Errorf("kind %q is not supported yet", m.Kind)
	}
	if m.Kind != "channel" {
		return Manifest{}, fmt.Errorf("invalid kind %q", m.Kind)
	}

	m.Command = strings.TrimSpace(m.Command)
	if m.Command == "" {
		return Manifest{}, fmt.Errorf("missing command")
	}

	m.Description = strings.TrimSpace(m.Description)
	return m, nil
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
