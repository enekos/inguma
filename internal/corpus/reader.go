package corpus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/enekos/inguma/internal/manifest"
)

// ReadTool loads corpus/<slug>/{manifest.json,index.md}.
// The returned readme bytes are the raw index.md file contents (with frontmatter).
func ReadTool(root, slug string) (manifest.Tool, []byte, error) {
	dir := filepath.Join(root, slug)
	mf, err := os.ReadFile(filepath.Join(dir, "manifest.json"))
	if err != nil {
		return manifest.Tool{}, nil, err
	}
	var t manifest.Tool
	dec := json.NewDecoder(bytes.NewReader(mf))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&t); err != nil {
		return manifest.Tool{}, nil, fmt.Errorf("corpus: parse manifest %s: %w", slug, err)
	}
	readme, err := os.ReadFile(filepath.Join(dir, "index.md"))
	if err != nil {
		return manifest.Tool{}, nil, err
	}
	return t, readme, nil
}

// ReadIndex loads corpus/_index.json.
func ReadIndex(root string) ([]IndexEntry, error) {
	data, err := os.ReadFile(filepath.Join(root, "_index.json"))
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Tools []IndexEntry `json:"tools"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		return nil, fmt.Errorf("corpus: parse _index.json: %w", err)
	}
	return parsed.Tools, nil
}
