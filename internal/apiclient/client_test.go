package apiclient

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetTool(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/tools/my-tool" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"slug": "my-tool",
			"manifest": map[string]any{
				"name": "my-tool",
				"kind": "mcp",
			},
			"readme": "# hi",
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	tr, err := c.GetTool("my-tool")
	if err != nil {
		t.Fatal(err)
	}
	if tr.Slug != "my-tool" || tr.Manifest.Name != "my-tool" {
		t.Errorf("got = %+v", tr)
	}
}

func TestGetTool_notFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"not_found"}`, http.StatusNotFound)
	}))
	defer srv.Close()
	_, err := New(srv.URL).GetTool("x")
	if err == nil {
		t.Fatal("want error")
	}
}

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("q") != "hello" {
			t.Errorf("q = %q", r.URL.Query().Get("q"))
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"slug": "a", "score": 0.9, "tool": map[string]any{"display_name": "A", "description": "first"}},
			},
		})
	}))
	defer srv.Close()

	hits, err := New(srv.URL).Search("hello", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Slug != "a" {
		t.Errorf("got = %+v", hits)
	}
}
