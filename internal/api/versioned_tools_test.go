package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enekos/inguma/internal/corpus"
)

func seedVersionedCorpus(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for _, v := range []string{"v1.0.0", "v1.1.0"} {
		mf := []byte(`{"name":"bar","display_name":"Bar","description":"x","readme":"README.md","license":"MIT","kind":"mcp","mcp":{"transport":"stdio","command":"true"},"compatibility":{"harnesses":["claude-code"],"platforms":["darwin"]}}`)
		if err := corpus.WriteVersion(dir, corpus.VersionedEntry{
			Owner: "foo", Slug: "bar", Version: v,
			ManifestJSON: mf, IndexMD: []byte("---\nslug: bar\n---\n# bar"), ArtifactSHA: "sha-" + v,
		}); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func newAPI(t *testing.T, corpusDir string) http.Handler {
	t.Helper()
	s := &Server{CorpusDir: corpusDir}
	return s.Handler()
}

func TestGetVersionedTool(t *testing.T) {
	h := newAPI(t, seedVersionedCorpus(t))
	req := httptest.NewRequest("GET", "/api/tools/@foo/bar", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		Owner         string   `json:"owner"`
		Slug          string   `json:"slug"`
		LatestVersion string   `json:"latest_version"`
		Version       string   `json:"version"`
		Versions      []string `json:"versions"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body.LatestVersion != "v1.1.0" || body.Version != "v1.1.0" {
		t.Fatalf("latest/version mismatch: %+v", body)
	}
	if len(body.Versions) != 2 {
		t.Fatalf("versions=%v", body.Versions)
	}
}

func TestGetVersionList(t *testing.T) {
	h := newAPI(t, seedVersionedCorpus(t))
	req := httptest.NewRequest("GET", "/api/tools/@foo/bar/versions", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestGetVersionedToolAtVersion(t *testing.T) {
	h := newAPI(t, seedVersionedCorpus(t))
	req := httptest.NewRequest("GET", "/api/tools/@foo/bar/@v1.0.0", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGetVersionedToolNotFound(t *testing.T) {
	h := newAPI(t, t.TempDir())
	req := httptest.NewRequest("GET", "/api/tools/@foo/bar", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestGetVersionedToolBadVersion(t *testing.T) {
	h := newAPI(t, seedVersionedCorpus(t))
	req := httptest.NewRequest("GET", "/api/tools/@foo/bar/@not-a-version", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 400 {
		t.Fatalf("status=%d", rec.Code)
	}
}
