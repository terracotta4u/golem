package extension

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func List(root string) ([]Manifest, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out []Manifest
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, e.Name())
		m, err := Load(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if m.Name != e.Name() {
			return nil, fmt.Errorf("extension %s: manifest name %q does not match", e.Name(), m.Name)
		}
		m.Dir = dir
		out = append(out, m)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
