package api

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

func TestInstallVersionedLatest(t *testing.T) {
	h := newAPI(t, seedVersionedCorpus(t))
	req := httptest.NewRequest("GET", "/api/install/@foo/bar", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body struct {
		ResolvedVersion string `json:"resolved_version"`
		SHA256          string `json:"sha256"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.ResolvedVersion != "v1.1.0" {
		t.Fatalf("got %s", body.ResolvedVersion)
	}
	if body.SHA256 == "" {
		t.Fatal("no sha")
	}
}

func TestInstallVersionedExplicit(t *testing.T) {
	h := newAPI(t, seedVersionedCorpus(t))
	req := httptest.NewRequest("GET", "/api/install/@foo/bar/@v1.0.0", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
	var body struct {
		ResolvedVersion string `json:"resolved_version"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.ResolvedVersion != "v1.0.0" {
		t.Fatalf("got %s", body.ResolvedVersion)
	}
}

func TestInstallVersionedRange(t *testing.T) {
	h := newAPI(t, seedVersionedCorpus(t))
	req := httptest.NewRequest("GET", "/api/install/@foo/bar?range=^1.0", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 200 {
		t.Fatalf("status=%d", rec.Code)
	}
	var body struct {
		ResolvedVersion string `json:"resolved_version"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&body)
	if body.ResolvedVersion != "v1.1.0" {
		t.Fatalf("got %s", body.ResolvedVersion)
	}
}
