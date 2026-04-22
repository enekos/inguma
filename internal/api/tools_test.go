package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
