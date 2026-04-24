package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/enekos/inguma/internal/auth"
)

// sessionTTL bounds how long a web or device session stays valid
// before the user must re-auth. Short enough to limit token-theft
// blast radius; long enough to not be annoying.
const sessionTTL = 30 * 24 * time.Hour

// AuthDeps bundles the optional auth dependencies wired by the server.
// Handlers no-op when these are nil (tests that don't need auth skip
// wiring them).
type AuthDeps struct {
	Store  *auth.Store
	GH     auth.GitHub
	Admins []string
}

// NewAuthDeps is a convenience constructor for callers outside the
// package (cmd/apid).
func NewAuthDeps(store *auth.Store, gh auth.GitHub, admins []string) *AuthDeps {
	return &AuthDeps{Store: store, GH: gh, Admins: admins}
}

// currentSession reads the Authorization or inguma_session cookie and
// resolves it to a Session. Returns nil, nil for anonymous requests.
func (s *Server) currentSession(r *http.Request) (*auth.Session, error) {
	if s.Auth == nil || s.Auth.Store == nil {
		return nil, nil
	}
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return s.Auth.Store.Lookup(strings.TrimPrefix(h, "Bearer "))
	}
	c, err := r.Cookie("inguma_session")
	if err != nil || c.Value == "" {
		return nil, nil
	}
	return s.Auth.Store.Lookup(c.Value)
}

// /api/me — returns the current session or null.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if s.Auth == nil || s.Auth.Store == nil {
		writeError(w, http.StatusNotFound, "auth_disabled", "authentication not configured")
		return
	}
	sess, err := s.currentSession(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, sess)
}

// /api/auth/logout
func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if s.Auth == nil || s.Auth.Store == nil {
		writeError(w, http.StatusNotFound, "auth_disabled", "authentication not configured")
		return
	}
	sess, _ := s.currentSession(r)
	if sess != nil {
		_ = s.Auth.Store.Delete(sess.Token)
	}
	http.SetCookie(w, &http.Cookie{Name: "inguma_session", Value: "", MaxAge: -1, Path: "/"})
	w.WriteHeader(http.StatusNoContent)
}

// /api/auth/device/start — CLI asks server to kick off a device flow.
func (s *Server) handleDeviceStart(w http.ResponseWriter, r *http.Request) {
	if s.Auth == nil || s.Auth.Store == nil || s.Auth.GH == nil {
		writeError(w, http.StatusNotFound, "auth_disabled", "authentication not configured")
		return
	}
	start, err := s.Auth.Store.CreateDeviceCode(s.Auth.GH)
	if err != nil {
		writeError(w, http.StatusBadGateway, "github_device_start_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, start)
}

// /api/auth/device/poll?device_code=...
func (s *Server) handleDevicePoll(w http.ResponseWriter, r *http.Request) {
	if s.Auth == nil || s.Auth.Store == nil || s.Auth.GH == nil {
		writeError(w, http.StatusNotFound, "auth_disabled", "authentication not configured")
		return
	}
	code := r.URL.Query().Get("device_code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing_device_code", "device_code query param required")
		return
	}
	tok, status, err := s.Auth.Store.PollDevice(s.Auth.GH, code, sessionTTL)
	if err != nil {
		writeError(w, http.StatusBadGateway, "poll_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": status, "token": tok})
}

// /api/auth/github/callback?code=... — web-OAuth completion URL.
func (s *Server) handleGitHubCallback(w http.ResponseWriter, r *http.Request) {
	if s.Auth == nil || s.Auth.Store == nil || s.Auth.GH == nil {
		writeError(w, http.StatusNotFound, "auth_disabled", "authentication not configured")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing_code", "code query param required")
		return
	}
	ghTok, err := s.Auth.GH.ExchangeCode(code)
	if err != nil {
		writeError(w, http.StatusBadGateway, "exchange_failed", err.Error())
		return
	}
	login, id, err := s.Auth.GH.GetUser(ghTok)
	if err != nil {
		writeError(w, http.StatusBadGateway, "get_user_failed", err.Error())
		return
	}
	orgs, _ := s.Auth.GH.ListOrgs(ghTok)
	sess, err := s.Auth.Store.CreateSession(login, id, orgs, sessionTTL)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "create_session_failed", err.Error())
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "inguma_session",
		Value:    sess.Token,
		Path:     "/",
		HttpOnly: true,
		Secure:   r.TLS != nil,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionTTL / time.Second),
	})
	// If the caller was a browser, redirect home. JSON clients get JSON.
	if strings.Contains(r.Header.Get("Accept"), "text/html") {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "token": sess.Token})
}

// requireScope authenticates the request and verifies `manage:@owner`
// (or admin). Returns the session on success, writes an error and
// returns nil on failure.
func (s *Server) requireManage(w http.ResponseWriter, r *http.Request, owner string) *auth.Session {
	sess, err := s.currentSession(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup_failed", err.Error())
		return nil
	}
	if sess == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "sign in required")
		return nil
	}
	if !sess.CanManage(owner) {
		writeError(w, http.StatusForbidden, "forbidden", "manage:@"+owner+" scope required")
		return nil
	}
	return sess
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) *auth.Session {
	sess, err := s.currentSession(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "lookup_failed", err.Error())
		return nil
	}
	if sess == nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "sign in required")
		return nil
	}
	if !sess.IsAdmin() {
		writeError(w, http.StatusForbidden, "forbidden", "admin scope required")
		return nil
	}
	return sess
}

// decodeBody reads a JSON body into out, capping size at 64 KiB.
func decodeBody(r *http.Request, out any) error {
	lim := io.LimitReader(r.Body, 64*1024)
	dec := json.NewDecoder(lim)
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return errors.New("invalid request body: " + err.Error())
	}
	return nil
}
