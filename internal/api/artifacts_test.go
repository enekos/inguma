package api

import (
	"bytes"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/enekos/inguma/internal/artifacts"
	"github.com/enekos/inguma/internal/db"
)

func TestGetArtifact(t *testing.T) {
	dir := t.TempDir()
	store := artifacts.NewFSStore(filepath.Join(dir, "artifacts"))
	ref := artifacts.Ref{Owner: "foo", Slug: "bar", Version: "v1.0.0"}
	body := []byte("tarball-contents")
	if _, err := store.Put(ref, bytes.NewReader(body)); err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(filepath.Join(dir, "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	s := &Server{Store: store, DB: database}
	h := s.Handler()

	req := httptest.NewRequest("GET", "/api/artifacts/@foo/bar/@v1.0.0", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Content-Type") != "application/gzip" {
		t.Fatalf("ct=%s", rec.Header().Get("Content-Type"))
	}
	if rec.Header().Get("X-Inguma-SHA256") == "" {
		t.Fatal("missing sha header")
	}
	if !bytes.Equal(rec.Body.Bytes(), body) {
		t.Fatalf("body mismatch")
	}

	// Second request increments download count.
	req2 := httptest.NewRequest("GET", "/api/artifacts/@foo/bar/@v1.0.0", nil)
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req2)
	n, _ := database.DownloadCount("foo", "bar", "v1.0.0")
	if n != 2 {
		t.Fatalf("count=%d want 2", n)
	}
}

func TestGetArtifactNoStore(t *testing.T) {
	s := &Server{}
	h := s.Handler()
	req := httptest.NewRequest("GET", "/api/artifacts/@foo/bar/@v1.0.0", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 503 {
		t.Fatalf("status=%d", rec.Code)
	}
}

func TestGetArtifactNotFound(t *testing.T) {
	dir := t.TempDir()
	store := artifacts.NewFSStore(filepath.Join(dir, "artifacts"))
	s := &Server{Store: store}
	h := s.Handler()
	req := httptest.NewRequest("GET", "/api/artifacts/@foo/bar/@v9.9.9", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 404 {
		t.Fatalf("status=%d", rec.Code)
	}
}
