package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCategories(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	var parsed struct {
		Categories []struct {
			Name  string `json:"name"`
			Count int    `json:"count"`
		} `json:"categories"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	// fixture corpus has search (1) and git (1)
	if len(parsed.Categories) != 2 {
		t.Fatalf("got %d categories: %+v", len(parsed.Categories), parsed.Categories)
	}
	// sorted alphabetically
	if parsed.Categories[0].Name != "git" || parsed.Categories[1].Name != "search" {
		t.Errorf("order = %+v", parsed.Categories)
	}
}

func TestBrowseAll(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/tools", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("code = %d", w.Code)
	}
	var parsed struct {
		Tools []struct {
			Slug string `json:"slug"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed.Tools) != 2 {
		t.Errorf("got %d tools", len(parsed.Tools))
	}
}

func TestBrowseByCategory(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/tools?category=search", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatal(w.Code)
	}
	var parsed struct {
		Tools []struct {
			Slug string `json:"slug"`
		} `json:"tools"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &parsed)
	if len(parsed.Tools) != 1 || parsed.Tools[0].Slug != "tool-a" {
		t.Errorf("got = %+v", parsed.Tools)
	}
}
