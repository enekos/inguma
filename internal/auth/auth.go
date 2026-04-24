// Package auth implements GitHub OAuth web sessions and device-flow
// authentication for the inguma CLI.
//
// The package deliberately hides the GitHub HTTP transport behind a
// GitHub interface so tests can substitute a fake. Live wiring lives in
// NewGitHub (see github.go).
package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Scope names used across the codebase.
const (
	ScopeRead   = "read"
	ScopeManage = "manage" // paired with @owner: "manage:@foo"
	ScopeAdmin  = "admin"
)

// Session is a row in the sessions table.
type Session struct {
	Token     string    `json:"-"`
	GHUser    string    `json:"gh_user"`
	GHID      int64     `json:"gh_id"`
	Scopes    []string  `json:"scopes"`
	Orgs      []string  `json:"orgs"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Store persists sessions and device codes.
type Store struct {
	db     *sql.DB
	admins map[string]bool
}

// NewStore wraps the SQLite handle. admins is the list of GitHub
// logins that automatically receive the admin scope.
func NewStore(db *sql.DB, admins []string) *Store {
	m := make(map[string]bool, len(admins))
	for _, a := range admins {
		m[strings.ToLower(a)] = true
	}
	return &Store{db: db, admins: m}
}

// GitHub is the subset of the GitHub API the auth package uses.
type GitHub interface {
	// ExchangeCode swaps a web-OAuth code for an access token.
	ExchangeCode(code string) (string, error)
	// StartDeviceFlow returns the user_code + device_code pair.
	StartDeviceFlow() (DeviceStart, error)
	// PollDeviceFlow exchanges a device_code for a token.
	// Returns ("", "authorization_pending", nil) while pending.
	PollDeviceFlow(deviceCode string) (token string, slowDown bool, err error)
	// GetUser returns the login + id for a token.
	GetUser(token string) (login string, id int64, err error)
	// ListOrgs returns the org logins visible to the token (read:org).
	ListOrgs(token string) ([]string, error)
}

// DeviceStart is the payload we hand back to the CLI to present to
// the user on StartDeviceFlow.
type DeviceStart struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	Interval        int    `json:"interval"`
	ExpiresIn       int    `json:"expires_in"`
}

// randomToken returns a 48-byte hex string.
func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateSession inserts a new session row for the given GitHub user,
// including their org memberships. Scopes are: read + manage:<org> for
// each org + admin if the login is in the admin list.
func (s *Store) CreateSession(login string, id int64, orgs []string, ttl time.Duration) (*Session, error) {
	tok, err := randomToken()
	if err != nil {
		return nil, err
	}
	scopes := []string{ScopeRead, "manage:@" + strings.ToLower(login)}
	for _, o := range orgs {
		scopes = append(scopes, "manage:@"+strings.ToLower(o))
	}
	if s.admins[strings.ToLower(login)] {
		scopes = append(scopes, ScopeAdmin)
	}
	now := time.Now().UTC()
	expires := now.Add(ttl)
	_, err = s.db.Exec(
		`INSERT INTO sessions(token, gh_user, gh_id, scopes, orgs, created_at, expires_at) VALUES (?,?,?,?,?,?,?)`,
		tok, strings.ToLower(login), id, strings.Join(scopes, ","), strings.Join(orgs, ","),
		now.Format(time.RFC3339), expires.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	return &Session{Token: tok, GHUser: strings.ToLower(login), GHID: id, Scopes: scopes, Orgs: orgs, ExpiresAt: expires}, nil
}

// Lookup returns the session for a token or nil if missing/expired.
func (s *Store) Lookup(token string) (*Session, error) {
	if token == "" {
		return nil, nil
	}
	var (
		ghUser, scopes, orgs, expiresAt string
		ghID                            int64
	)
	err := s.db.QueryRow(
		`SELECT gh_user, gh_id, scopes, orgs, expires_at FROM sessions WHERE token = ?`, token,
	).Scan(&ghUser, &ghID, &scopes, &orgs, &expiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	exp, _ := time.Parse(time.RFC3339, expiresAt)
	if exp.Before(time.Now().UTC()) {
		_, _ = s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
		return nil, nil
	}
	return &Session{
		Token:     token,
		GHUser:    ghUser,
		GHID:      ghID,
		Scopes:    splitNonEmpty(scopes, ","),
		Orgs:      splitNonEmpty(orgs, ","),
		ExpiresAt: exp,
	}, nil
}

// Delete removes a session.
func (s *Store) Delete(token string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE token = ?`, token)
	return err
}

// CanManage returns true iff the session has manage rights over @owner.
func (sess *Session) CanManage(owner string) bool {
	if sess == nil {
		return false
	}
	want := "manage:@" + strings.ToLower(owner)
	for _, sc := range sess.Scopes {
		if sc == want || sc == ScopeAdmin {
			return true
		}
	}
	return false
}

// IsAdmin reports whether the session carries the admin scope.
func (sess *Session) IsAdmin() bool {
	if sess == nil {
		return false
	}
	for _, sc := range sess.Scopes {
		if sc == ScopeAdmin {
			return true
		}
	}
	return false
}

// ---- Device flow ----------------------------------------------------

// CreateDeviceCode reserves a pending device code.
func (s *Store) CreateDeviceCode(gh GitHub) (DeviceStart, error) {
	// The GitHub device flow is the source of truth for user_code /
	// device_code values. We mirror them locally so the CLI can poll
	// against our server rather than GitHub directly.
	d, err := gh.StartDeviceFlow()
	if err != nil {
		return DeviceStart{}, err
	}
	now := time.Now().UTC()
	exp := now.Add(time.Duration(d.ExpiresIn) * time.Second)
	if d.ExpiresIn <= 0 {
		exp = now.Add(15 * time.Minute)
	}
	_, err = s.db.Exec(
		`INSERT INTO device_codes(device_code, user_code, created_at, expires_at, interval_s) VALUES (?,?,?,?,?)`,
		d.DeviceCode, d.UserCode, now.Format(time.RFC3339), exp.Format(time.RFC3339), d.Interval,
	)
	if err != nil {
		return DeviceStart{}, err
	}
	return d, nil
}

// PollDevice exchanges a device_code for a session token if the user
// has completed auth upstream. Returns a blank token + ("pending", nil)
// while waiting.
func (s *Store) PollDevice(gh GitHub, deviceCode string, sessionTTL time.Duration) (token, status string, err error) {
	var (
		existing   sql.NullString
		expiresStr string
	)
	err = s.db.QueryRow(`SELECT token, expires_at FROM device_codes WHERE device_code = ?`, deviceCode).Scan(&existing, &expiresStr)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "invalid", nil
	}
	if err != nil {
		return "", "", err
	}
	exp, _ := time.Parse(time.RFC3339, expiresStr)
	if exp.Before(time.Now().UTC()) {
		return "", "expired_token", nil
	}
	if existing.Valid && existing.String != "" {
		return existing.String, "ok", nil
	}
	ghToken, slow, err := gh.PollDeviceFlow(deviceCode)
	if err != nil {
		return "", "", err
	}
	if ghToken == "" {
		if slow {
			return "", "slow_down", nil
		}
		return "", "authorization_pending", nil
	}
	login, id, err := gh.GetUser(ghToken)
	if err != nil {
		return "", "", fmt.Errorf("fetch user: %w", err)
	}
	orgs, err := gh.ListOrgs(ghToken)
	if err != nil {
		// Don't fail auth if the org listing is unavailable; the user
		// simply gets no manage:@org scopes this session.
		orgs = nil
	}
	sess, err := s.CreateSession(login, id, orgs, sessionTTL)
	if err != nil {
		return "", "", err
	}
	if _, err := s.db.Exec(`UPDATE device_codes SET token = ? WHERE device_code = ?`, sess.Token, deviceCode); err != nil {
		return "", "", err
	}
	return sess.Token, "ok", nil
}

// ---- helpers --------------------------------------------------------

func splitNonEmpty(s, sep string) []string {
	if s == "" {
		return nil
	}
	out := strings.Split(s, sep)
	clean := out[:0]
	for _, x := range out {
		if x != "" {
			clean = append(clean, x)
		}
	}
	return clean
}

// SessionJSON returns a redacted JSON view suitable for /api/me.
func SessionJSON(sess *Session) ([]byte, error) {
	if sess == nil {
		return []byte(`null`), nil
	}
	return json.Marshal(sess)
}
