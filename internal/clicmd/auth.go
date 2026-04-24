// Package-level file for login/logout/whoami commands (Track B).
package clicmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/enekos/inguma/internal/apiclient"
)

// TokenFile is ~/.inguma/token; mode 0600.
func TokenFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".inguma", "token")
}

// storedToken reads the persisted bearer token.
type storedToken struct {
	Token  string `json:"token"`
	GHUser string `json:"gh_user,omitempty"`
}

func loadToken(path string) (*storedToken, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &storedToken{}, nil
	}
	if err != nil {
		return nil, err
	}
	var t storedToken
	if err := json.Unmarshal(b, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func saveToken(path string, t *storedToken) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o600)
}

// AttachSavedToken loads the token file and configures the client.
// Missing file is a no-op.
func AttachSavedToken(c *apiclient.Client) error {
	t, err := loadToken(TokenFile())
	if err != nil {
		return err
	}
	if t.Token != "" {
		c.SetToken(t.Token)
	}
	return nil
}

type LoginDeps struct {
	API    *apiclient.Client
	Stdout io.Writer
	Sleep  func(time.Duration) // test seam; defaults to time.Sleep
}

// Login runs the full device flow: ask the server to start it, present
// the user_code URL, poll until a token shows up, persist it.
func Login(ctx context.Context, deps LoginDeps) error {
	if deps.Sleep == nil {
		deps.Sleep = time.Sleep
	}
	start, err := deps.API.StartDeviceFlow()
	if err != nil {
		return fmt.Errorf("login: %w", err)
	}
	fmt.Fprintf(deps.Stdout, "Open %s in your browser and enter: %s\n", start.VerificationURI, start.UserCode)
	fmt.Fprintln(deps.Stdout, "Waiting for authorization…")
	interval := start.Interval
	if interval <= 0 {
		interval = 5
	}
	deadline := time.Now().Add(15 * time.Minute)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("login: device code expired")
		}
		deps.Sleep(time.Duration(interval) * time.Second)
		tok, status, err := deps.API.PollDeviceFlow(start.DeviceCode)
		if err != nil {
			return fmt.Errorf("login: poll: %w", err)
		}
		switch status {
		case "ok":
			deps.API.SetToken(tok)
			me, _ := deps.API.Me()
			login := ""
			if me != nil {
				login = me.GHUser
			}
			if err := saveToken(TokenFile(), &storedToken{Token: tok, GHUser: login}); err != nil {
				return fmt.Errorf("login: save token: %w", err)
			}
			fmt.Fprintf(deps.Stdout, "Logged in as @%s\n", login)
			return nil
		case "authorization_pending":
			continue
		case "slow_down":
			interval += 5
			continue
		case "expired_token":
			return fmt.Errorf("login: device code expired, run `inguma login` again")
		case "invalid":
			return fmt.Errorf("login: server rejected device code")
		default:
			return fmt.Errorf("login: unexpected status %q", status)
		}
	}
}

type LogoutDeps struct {
	API    *apiclient.Client
	Stdout io.Writer
}

// Logout removes the server-side session and deletes the local token.
func Logout(_ context.Context, deps LogoutDeps) error {
	_ = AttachSavedToken(deps.API)
	_ = deps.API.Logout()
	_ = os.Remove(TokenFile())
	fmt.Fprintln(deps.Stdout, "Logged out.")
	return nil
}

type WhoamiDeps struct {
	API    *apiclient.Client
	Stdout io.Writer
}

// Whoami prints the current session's gh user and scopes.
func Whoami(_ context.Context, deps WhoamiDeps) error {
	if err := AttachSavedToken(deps.API); err != nil {
		return err
	}
	me, err := deps.API.Me()
	if err != nil {
		return err
	}
	if me == nil {
		fmt.Fprintln(deps.Stdout, "Not logged in. Run `inguma login`.")
		return nil
	}
	fmt.Fprintf(deps.Stdout, "@%s (scopes: %v)\n", me.GHUser, me.Scopes)
	return nil
}
