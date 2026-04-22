package marrow

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearch(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/search" {
			t.Errorf("bad method/path: %s %s", r.Method, r.URL.Path)
		}
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"slug": "tool-a", "score": 0.9},
				{"slug": "tool-b", "score": 0.7},
			},
		})
	}))
	defer srv.Close()

	c := New(srv.URL)
	res, err := c.Search(context.Background(), Query{Q: "hello", Limit: 10, Lang: "en"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) != 2 || res[0].Slug != "tool-a" {
		t.Errorf("res = %+v", res)
	}
	if !strings.Contains(gotBody, `"q":"hello"`) || !strings.Contains(gotBody, `"limit":10`) {
		t.Errorf("request body = %s", gotBody)
	}
}

func TestSearch_errorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := New(srv.URL).Search(context.Background(), Query{Q: "x"})
	if err == nil {
		t.Fatal("want error")
	}
}
