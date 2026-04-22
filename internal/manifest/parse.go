package manifest

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Parse reads a manifest from YAML bytes.
// Unknown top-level keys are an error (strict decoding).
func Parse(data []byte) (*Tool, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	var t Tool
	if err := dec.Decode(&t); err != nil {
		return nil, fmt.Errorf("manifest: parse: %w", err)
	}
	return &t, nil
}

// ParseFile reads and parses a manifest from disk.
func ParseFile(path string) (*Tool, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("manifest: read %s: %w", path, err)
	}
	return Parse(data)
}
