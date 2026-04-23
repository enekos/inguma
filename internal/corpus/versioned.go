package corpus

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/enekos/inguma/internal/versioning"
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

// ListVersions returns canonical version strings present on disk for
// @owner/slug, sorted ascending by semver. Raw directory entries that
// are not valid semver are silently dropped.
func ListVersions(root, owner, slug string) ([]string, error) {
	dir := filepath.Join(root, owner, slug, "versions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	raw := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			raw = append(raw, e.Name())
		}
	}
	vs := versioning.ScanTags(raw)
	out := make([]string, len(vs))
	for i, v := range vs {
		out[i] = v.Canonical()
	}
	return out, nil
}

// ReadVersion returns manifest, index.md, and artifact sha for a specific version.
func ReadVersion(root, owner, slug, version string) (manifestJSON []byte, indexMD []byte, sha string, err error) {
	base := filepath.Join(root, owner, slug, "versions", version)
	m, err := os.ReadFile(filepath.Join(base, "manifest.json"))
	if err != nil {
		return nil, nil, "", err
	}
	idx, err := os.ReadFile(filepath.Join(base, "index.md"))
	if err != nil {
		return nil, nil, "", err
	}
	shab, err := os.ReadFile(filepath.Join(base, "artifact.sha256"))
	if err != nil {
		return nil, nil, "", err
	}
	return m, idx, string(shab), nil
}

// HasVersion reports whether a specific version exists in the corpus.
func HasVersion(root, owner, slug, version string) bool {
	base := filepath.Join(root, owner, slug, "versions", version)
	_, err := os.Stat(filepath.Join(base, "manifest.json"))
	return err == nil
}
