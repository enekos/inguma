package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enekos/agentpop/internal/marrow"
)

type stubMarrow struct {
	results []marrow.Result
	err     error
	lastQ   marrow.Query
}

func (s *stubMarrow) Search(ctx context.Context, q marrow.Query) ([]marrow.Result, error) {
	s.lastQ = q
	return s.results, s.err
}

func TestSearch_hydratesAndFilters(t *testing.T) {
	stub := &stubMarrow{results: []marrow.Result{
		{Slug: "tool-a", Score: 0.9},
		{Slug: "tool-b", Score: 0.7},
	}}
	s := newTestServer(t)
	s.Marrow = stub

	r := httptest.NewRequest(http.MethodGet, "/api/search?q=hello&kind=mcp", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	if stub.lastQ.Q != "hello" {
		t.Errorf("marrow got q = %q", stub.lastQ.Q)
	}
	var parsed struct {
		Results []struct {
			Slug  string  `json:"slug"`
			Score float64 `json:"score"`
			Tool  struct {
				DisplayName string `json:"display_name"`
				Kind        string `json:"kind"`
			} `json:"tool"`
		} `json:"results"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	// tool-b is kind=cli — the kind=mcp filter must drop it.
	if len(parsed.Results) != 1 {
		t.Fatalf("got %d results: %+v", len(parsed.Results), parsed.Results)
	}
	if parsed.Results[0].Slug != "tool-a" || parsed.Results[0].Tool.Kind != "mcp" {
		t.Errorf("got = %+v", parsed.Results[0])
	}
}

func TestSearch_marrowDown(t *testing.T) {
	s := newTestServer(t)
	s.Marrow = &stubMarrow{err: errors.New("boom")}

	r := httptest.NewRequest(http.MethodGet, "/api/search?q=x", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("code = %d", w.Code)
	}
}

func TestSearch_emptyQuery(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/search", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusBadRequest {
		t.Errorf("code = %d", w.Code)
	}
}
