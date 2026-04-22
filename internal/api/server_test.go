package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/enekos/agentpop/internal/adapters/all"
	"github.com/enekos/agentpop/internal/marrow"
)

// fakeMarrow is a no-op Marrow client used in unit tests that don't hit search.
type fakeMarrow struct{}

func (fakeMarrow) Search(ctx context.Context, q marrow.Query) ([]marrow.Result, error) {
	return nil, nil
}

// newTestServer builds a Server rooted at the fixture corpus.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	return &Server{
		CorpusDir: "testdata/corpus",
		Marrow:    fakeMarrow{},
		Adapters:  all.Default(),
	}
}

func TestRoutes_unknownReturns404(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/nope", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d", w.Code)
	}
}
