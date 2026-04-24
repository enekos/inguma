package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/enekos/inguma/internal/adapters/all"
	"github.com/enekos/inguma/internal/advisories"
	"github.com/enekos/inguma/internal/auth"
	"github.com/enekos/inguma/internal/db"
	"github.com/enekos/inguma/internal/pkgstate"
)

// fakeGH minimally satisfies auth.GitHub for API tests (no token exchange).
type fakeGH struct{}

func (fakeGH) ExchangeCode(string) (string, error)          { return "", nil }
func (fakeGH) StartDeviceFlow() (auth.DeviceStart, error)   { return auth.DeviceStart{}, nil }
func (fakeGH) PollDeviceFlow(string) (string, bool, error)  { return "", false, nil }
func (fakeGH) GetUser(string) (string, int64, error)        { return "", 0, nil }
func (fakeGH) ListOrgs(string) ([]string, error)            { return nil, nil }

func newAuthedServer(t *testing.T, admins ...string) (*Server, *auth.Store, *db.DB) {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	store := auth.NewStore(d.SQL(), admins)
	s := &Server{
		CorpusDir:  "testdata/corpus",
		Marrow:     fakeMarrow{},
		Adapters:   all.Default(),
		DB:         d,
		PkgState:   pkgstate.NewStore(d.SQL()),
		Advisories: advisories.NewStore(d.SQL()),
		Auth:       NewAuthDeps(store, fakeGH{}, admins),
	}
	return s, store, d
}

func TestMe_Unauthenticated(t *testing.T) {
	s, _, _ := newAuthedServer(t)
	r := httptest.NewRequest("GET", "/api/me", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	if b := w.Body.String(); b != "null\n" {
		t.Fatalf("want null, got %q", b)
	}
}

func TestYank_AuthRequired(t *testing.T) {
	s, _, _ := newAuthedServer(t)
	r := httptest.NewRequest("POST", "/api/tools/@foo/bar/@v1.0.0/yank", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestYank_HappyPath(t *testing.T) {
	s, store, _ := newAuthedServer(t)
	sess, _ := store.CreateSession("foo", 1, nil, defaultTTL())
	r := httptest.NewRequest("POST", "/api/tools/@foo/bar/@v1.0.0/yank", nil)
	r.Header.Set("Authorization", "Bearer "+sess.Token)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", w.Code, w.Body.String())
	}
	st, _ := s.PkgState.Get("foo", "bar", "v1.0.0")
	if !st.Yanked {
		t.Fatalf("expected yanked=true")
	}
}

func TestYank_WrongOwnerForbidden(t *testing.T) {
	s, store, _ := newAuthedServer(t)
	sess, _ := store.CreateSession("notfoo", 2, nil, defaultTTL())
	r := httptest.NewRequest("POST", "/api/tools/@foo/bar/@v1.0.0/yank", nil)
	r.Header.Set("Authorization", "Bearer "+sess.Token)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("code=%d", w.Code)
	}
}

func TestPublishAdvisory_AdminOnly(t *testing.T) {
	s, store, _ := newAuthedServer(t, "root")
	// Non-admin → 403.
	sess, _ := store.CreateSession("notroot", 1, nil, defaultTTL())
	body, _ := json.Marshal(map[string]any{
		"owner": "foo", "slug": "bar", "range": "<1.2.4",
		"severity": "high", "summary": "xss",
	})
	r := httptest.NewRequest("POST", "/api/advisories", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+sess.Token)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("nonadmin code=%d", w.Code)
	}
	// Admin → 201.
	admin, _ := store.CreateSession("root", 2, nil, defaultTTL())
	r = httptest.NewRequest("POST", "/api/advisories", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+admin.Token)
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("admin code=%d body=%s", w.Code, w.Body.String())
	}
}

func defaultTTL() time.Duration { return 24 * time.Hour }
