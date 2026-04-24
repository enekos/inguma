package auth

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/enekos/inguma/internal/db"
)

// fakeGH is a scripted GitHub backend used by the device-flow tests.
type fakeGH struct {
	nextStart  DeviceStart
	pollResult struct {
		token string
		slow  bool
		err   error
	}
	user struct {
		login string
		id    int64
		err   error
	}
	orgs []string
}

func (f *fakeGH) ExchangeCode(string) (string, error)   { return "tok", nil }
func (f *fakeGH) StartDeviceFlow() (DeviceStart, error) { return f.nextStart, nil }
func (f *fakeGH) PollDeviceFlow(string) (string, bool, error) {
	return f.pollResult.token, f.pollResult.slow, f.pollResult.err
}
func (f *fakeGH) GetUser(string) (string, int64, error) { return f.user.login, f.user.id, f.user.err }
func (f *fakeGH) ListOrgs(string) ([]string, error)     { return f.orgs, nil }

func openStore(t *testing.T, admins ...string) *Store {
	t.Helper()
	d, err := db.Open(filepath.Join(t.TempDir(), "t.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { d.Close() })
	return NewStore(d.SQL(), admins)
}

func TestCreateAndLookupSession(t *testing.T) {
	s := openStore(t, "alice")
	sess, err := s.CreateSession("Alice", 42, []string{"Cool-Org"}, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if sess.GHUser != "alice" {
		t.Fatalf("login=%q want alice", sess.GHUser)
	}
	if !sess.CanManage("alice") || !sess.CanManage("cool-org") {
		t.Fatalf("expected manage scopes on own + org, got %v", sess.Scopes)
	}
	if !sess.IsAdmin() {
		t.Fatalf("alice is in admins list, expected admin")
	}
	got, err := s.Lookup(sess.Token)
	if err != nil || got == nil || got.GHUser != "alice" {
		t.Fatalf("lookup: %v %+v", err, got)
	}
}

func TestLookupMissing(t *testing.T) {
	s := openStore(t)
	got, err := s.Lookup("nope")
	if err != nil || got != nil {
		t.Fatalf("want nil/nil, got %+v/%v", got, err)
	}
}

func TestSessionExpires(t *testing.T) {
	s := openStore(t)
	sess, err := s.CreateSession("bob", 1, nil, -1*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s.Lookup(sess.Token)
	if err != nil || got != nil {
		t.Fatalf("want expired -> nil, got %+v", got)
	}
}

func TestDeletesSession(t *testing.T) {
	s := openStore(t)
	sess, _ := s.CreateSession("c", 2, nil, time.Hour)
	if err := s.Delete(sess.Token); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Lookup(sess.Token)
	if got != nil {
		t.Fatalf("expected deleted")
	}
}

func TestDeviceFlowHappy(t *testing.T) {
	s := openStore(t)
	f := &fakeGH{
		nextStart: DeviceStart{DeviceCode: "dc", UserCode: "AB-CD", VerificationURI: "https://gh/dev", Interval: 5, ExpiresIn: 600},
	}
	start, err := s.CreateDeviceCode(f)
	if err != nil || start.DeviceCode != "dc" {
		t.Fatalf("start: %+v err=%v", start, err)
	}
	// First poll: pending.
	tok, status, err := s.PollDevice(f, "dc", time.Hour)
	if err != nil || tok != "" || status != "authorization_pending" {
		t.Fatalf("pending: tok=%q status=%q err=%v", tok, status, err)
	}
	// Now GH returns a token.
	f.pollResult.token = "gh-access"
	f.user.login = "dina"
	f.user.id = 99
	f.orgs = []string{"Acme"}
	tok, status, err = s.PollDevice(f, "dc", time.Hour)
	if err != nil || status != "ok" || tok == "" {
		t.Fatalf("ok: tok=%q status=%q err=%v", tok, status, err)
	}
	// Repeated polls return the same token without re-calling GH.
	f.pollResult.err = errors.New("should-not-be-called")
	tok2, status2, err := s.PollDevice(f, "dc", time.Hour)
	if err != nil || status2 != "ok" || tok2 != tok {
		t.Fatalf("idempotent: tok=%q status=%q err=%v", tok2, status2, err)
	}
}

func TestDeviceFlowUnknownCode(t *testing.T) {
	s := openStore(t)
	f := &fakeGH{}
	tok, status, err := s.PollDevice(f, "bogus", time.Hour)
	if err != nil || tok != "" || status != "invalid" {
		t.Fatalf("want invalid, got tok=%q status=%q err=%v", tok, status, err)
	}
}
