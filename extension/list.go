package extension

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func List(root string) ([]Project, error) {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var out []Project
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		dir := filepath.Join(root, e.Name())
		p, err := Load(dir)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if p.Name != e.Name() {
			return nil, fmt.Errorf("extension %s: project name %q does not match", e.Name(), p.Name)
		}
		p.Dir = dir
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
