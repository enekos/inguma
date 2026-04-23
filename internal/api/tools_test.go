package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTools_get(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/tools/tool-a", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var parsed struct {
		Slug     string `json:"slug"`
		Manifest struct {
			Name string `json:"name"`
		} `json:"manifest"`
		Readme string `json:"readme"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Slug != "tool-a" || parsed.Manifest.Name != "tool-a" {
		t.Errorf("got = %+v", parsed)
	}
	if !strings.Contains(parsed.Readme, "Demo readme body") {
		t.Errorf("readme missing body: %q", parsed.Readme)
	}
}

func TestTools_notFound(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/tools/does-not-exist", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d", w.Code)
	}
}

func TestTools_rejectsBadSlug(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/tools/..%2Fescape", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d", w.Code)
	}
}

func TestBareSlugRedirectsWhenUnique(t *testing.T) {
	dir := seedVersionedCorpus(t)
	h := newAPI(t, dir)
	req := httptest.NewRequest("GET", "/api/tools/bar", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("status=%d want 301", rec.Code)
	}
	if rec.Header().Get("Location") != "/api/tools/@foo/bar" {
		t.Fatalf("location=%q", rec.Header().Get("Location"))
	}
}

func TestBareSlugFallsBackToV1WhenNoVersionedEntry(t *testing.T) {
	// Empty versioned corpus: no @owner/baz exists. But set up a v1 corpus entry.
	dir := t.TempDir()
	// Write a valid v1 layout corpus/<slug>/{manifest.json,index.md}.
	slugDir := filepath.Join(dir, "baz")
	if err := os.MkdirAll(slugDir, 0o755); err != nil {
		t.Fatal(err)
	}
	mf := []byte(`{"name":"baz","display_name":"Baz","description":"x","readme":"README.md","license":"MIT","kind":"mcp","mcp":{"transport":"stdio","command":"true"},"compatibility":{"harnesses":["claude-code"],"platforms":["darwin"]}}`)
	if err := os.WriteFile(filepath.Join(slugDir, "manifest.json"), mf, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(slugDir, "index.md"), []byte("---\nslug: baz\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	h := newAPI(t, dir)
	req := httptest.NewRequest("GET", "/api/tools/baz", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestBareSlugNoMatchReturns404(t *testing.T) {
	h := newAPI(t, t.TempDir())
	req := httptest.NewRequest("GET", "/api/tools/nope", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("status=%d", rec.Code)
	}
}
