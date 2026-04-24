package apiclient

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SetToken attaches a bearer token to every outbound request.
func (c *Client) SetToken(tok string) { c.token = tok }

// Token returns the currently-set bearer token, if any.
func (c *Client) Token() string { return c.token }

// Me returns the current session or nil if the server reports none.
func (c *Client) Me() (*Me, error) {
	resp, err := c.do("GET", "/api/me", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("server has auth disabled")
	}
	if resp.StatusCode >= 300 {
		return nil, nil
	}
	var out Me
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil || out.GHUser == "" {
		return nil, err
	}
	return &out, nil
}

// Me mirrors /api/me.
type Me struct {
	GHUser    string    `json:"gh_user"`
	GHID      int64     `json:"gh_id"`
	Scopes    []string  `json:"scopes"`
	Orgs      []string  `json:"orgs"`
	ExpiresAt time.Time `json:"expires_at"`
}

// DeviceStart mirrors /api/auth/device/start.
type DeviceStart struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	VerificationURI string `json:"verification_uri"`
	Interval        int    `json:"interval"`
	ExpiresIn       int    `json:"expires_in"`
}

// StartDeviceFlow begins a device-flow login.
func (c *Client) StartDeviceFlow() (*DeviceStart, error) {
	resp, err := c.do("POST", "/api/auth/device/start", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("device start: %d %s", resp.StatusCode, b)
	}
	var out DeviceStart
	return &out, json.NewDecoder(resp.Body).Decode(&out)
}

// PollDeviceFlow is a single poll. Returns ("","authorization_pending",nil) while waiting.
func (c *Client) PollDeviceFlow(deviceCode string) (token, status string, err error) {
	resp, err := c.do("GET", "/api/auth/device/poll?device_code="+url.QueryEscape(deviceCode), nil)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("device poll: %d %s", resp.StatusCode, b)
	}
	var body struct {
		Token  string `json:"token"`
		Status string `json:"status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", "", err
	}
	return body.Token, body.Status, nil
}

// Logout deletes the session on the server. Best-effort.
func (c *Client) Logout() error {
	resp, err := c.do("POST", "/api/auth/logout", nil)
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}

// ---- Mutations ------------------------------------------------------

// Yank marks @owner/slug@version yanked.
func (c *Client) Yank(owner, slug, version string) error {
	path := fmt.Sprintf("/api/tools/%s/%s/%s/yank",
		url.PathEscape("@"+owner), url.PathEscape(slug), url.PathEscape("@"+version))
	return c.mutate(path, nil)
}

// Deprecate marks a version (or whole package when version is empty) deprecated.
func (c *Client) Deprecate(owner, slug, version, message string) error {
	path := fmt.Sprintf("/api/tools/%s/%s/deprecate",
		url.PathEscape("@"+owner), url.PathEscape(slug))
	return c.mutate(path, map[string]string{"version": version, "message": message})
}

// PublishAdvisory posts a new advisory (admin only).
func (c *Client) PublishAdvisory(owner, slug, rangeExpr, severity, summary string, refs []string) error {
	path := "/api/advisories"
	body := map[string]any{
		"owner": owner, "slug": slug, "range": rangeExpr,
		"severity": severity, "summary": summary, "refs": refs,
	}
	return c.mutate(path, body)
}

// AdvisoryRow mirrors one row of /api/advisories.
type AdvisoryRow struct {
	Owner    string   `json:"owner"`
	Slug     string   `json:"slug"`
	Range    string   `json:"range"`
	Severity string   `json:"severity"`
	Summary  string   `json:"summary"`
	Refs     []string `json:"refs"`
}

// PackageAdvisories lists advisories registered against @owner/slug.
func (c *Client) PackageAdvisories(owner, slug string) ([]AdvisoryRow, error) {
	path := fmt.Sprintf("/api/tools/%s/%s/advisories",
		url.PathEscape("@"+owner), url.PathEscape(slug))
	var rows []AdvisoryRow
	if err := c.getJSON(path, &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// mutate POSTs body as JSON and returns an error on non-2xx.
func (c *Client) mutate(path string, body any) error {
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			return err
		}
	}
	resp, err := c.do("POST", path, &buf)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s → %d: %s", path, resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}

// do builds an authenticated request.
func (c *Client) do(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return c.http.Do(req)
}
