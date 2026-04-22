package registry

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Entry is a single tool in the curated registry.
type Entry struct {
	Repo string `yaml:"repo"`
	Ref  string `yaml:"ref"`
}

type file struct {
	Tools []Entry `yaml:"tools"`
}

// Load reads and parses the registry manifest.
func Load(path string) ([]Entry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("registry: read %s: %w", path, err)
	}
	var f file
	if err := yaml.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf("registry: parse %s: %w", path, err)
	}
	for i, e := range f.Tools {
		if e.Repo == "" {
			return nil, fmt.Errorf("registry: entry %d missing repo", i)
		}
		if e.Ref == "" {
			f.Tools[i].Ref = "main"
		}
	}
	return f.Tools, nil
}
