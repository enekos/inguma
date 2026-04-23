package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// VersionedEntry is the input to WriteVersion. The caller is responsible
// for producing ManifestJSON (canonical) and IndexMD (frontmatter + body).
type VersionedEntry struct {
	Owner        string
	Slug         string
	Version      string
	ManifestJSON []byte
	IndexMD      []byte
	ArtifactSHA  string
}

// WriteVersion writes corpus/<owner>/<slug>/versions/<version>/{manifest.json,index.md,artifact.sha256}
// and updates corpus/<owner>/<slug>/latest.json.
func WriteVersion(root string, e VersionedEntry) error {
	base := filepath.Join(root, e.Owner, e.Slug, "versions", e.Version)
	if err := os.MkdirAll(base, 0o755); err != nil {
		return fmt.Errorf("corpus: mkdir %s: %w", base, err)
	}
	if err := atomicWrite(filepath.Join(base, "manifest.json"), e.ManifestJSON); err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(base, "index.md"), e.IndexMD); err != nil {
		return err
	}
	if err := atomicWrite(filepath.Join(base, "artifact.sha256"), []byte(e.ArtifactSHA)); err != nil {
		return err
	}
	latest := map[string]string{"owner": e.Owner, "slug": e.Slug, "version": e.Version}
	data, err := json.Marshal(latest)
	if err != nil {
		return err
	}
	return atomicWrite(filepath.Join(root, e.Owner, e.Slug, "latest.json"), data)
}

func atomicWrite(p string, body []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(p), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(body); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), p)
}
