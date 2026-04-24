package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// GitHubClient is the live GitHub OAuth + REST client.
// Construct via NewGitHub.
type GitHubClient struct {
	ClientID     string
	ClientSecret string
	HTTP         *http.Client
	OAuthRoot    string // defaults to https://github.com
	APIRoot      string // defaults to https://api.github.com
}

// NewGitHub builds a live client; fields may still be zero if the
// server runs without OAuth configured, in which case its methods
// return an error.
func NewGitHub(clientID, clientSecret string) *GitHubClient {
	return &GitHubClient{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		HTTP:         &http.Client{Timeout: 15 * time.Second},
		OAuthRoot:    "https://github.com",
		APIRoot:      "https://api.github.com",
	}
}

func (g *GitHubClient) configured() error {
	if g == nil || g.ClientID == "" || g.ClientSecret == "" {
		return errors.New("github oauth not configured (missing INGUMA_GH_CLIENT_ID/SECRET)")
	}
	return nil
}

func (g *GitHubClient) ExchangeCode(code string) (string, error) {
	if err := g.configured(); err != nil {
		return "", err
	}
	form := url.Values{}
	form.Set("client_id", g.ClientID)
	form.Set("client_secret", g.ClientSecret)
	form.Set("code", code)
	req, _ := http.NewRequest(http.MethodPost, g.OAuthRoot+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := g.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var body struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", err
	}
	if body.Error != "" {
		return "", fmt.Errorf("github oauth: %s: %s", body.Error, body.ErrorDesc)
	}
	if body.AccessToken == "" {
		return "", errors.New("github oauth: no access_token in response")
	}
	return body.AccessToken, nil
}

func (g *GitHubClient) StartDeviceFlow() (DeviceStart, error) {
	if err := g.configured(); err != nil {
		return DeviceStart{}, err
	}
	form := url.Values{}
	form.Set("client_id", g.ClientID)
	form.Set("scope", "read:user read:org")
	req, _ := http.NewRequest(http.MethodPost, g.OAuthRoot+"/login/device/code", strings.NewReader(form.Encode()))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := g.HTTP.Do(req)
	if err != nil {
		return DeviceStart{}, err
	}
	defer resp.Body.Close()
	var body struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return DeviceStart{}, err
	}
	if body.DeviceCode == "" {
		return DeviceStart{}, errors.New("github device flow: no device_code")
	}
	if body.Interval == 0 {
		body.Interval = 5
	}
	return DeviceStart{
		DeviceCode:      body.DeviceCode,
		UserCode:        body.UserCode,
		VerificationURI: body.VerificationURI,
		ExpiresIn:       body.ExpiresIn,
		Interval:        body.Interval,
	}, nil
}

func (g *GitHubClient) PollDeviceFlow(deviceCode string) (string, bool, error) {
	if err := g.configured(); err != nil {
		return "", false, err
	}
	form := url.Values{}
	form.Set("client_id", g.ClientID)
	form.Set("device_code", deviceCode)
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
	req, _ := http.NewRequest(http.MethodPost, g.OAuthRoot+"/login/oauth/access_token", strings.NewReader(form.Encode()))
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := g.HTTP.Do(req)
	if err != nil {
		return "", false, err
	}
	defer resp.Body.Close()
	var body struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", false, err
	}
	switch body.Error {
	case "":
		return body.AccessToken, false, nil
	case "authorization_pending":
		return "", false, nil
	case "slow_down":
		return "", true, nil
	default:
		return "", false, fmt.Errorf("github device flow: %s", body.Error)
	}
}

func (g *GitHubClient) doAuthed(token, method, path string, out any) error {
	req, _ := http.NewRequest(method, g.APIRoot+path, nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := g.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("github %s: %s", path, strings.TrimSpace(string(b)))
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (g *GitHubClient) GetUser(token string) (string, int64, error) {
	var body struct {
		Login string `json:"login"`
		ID    int64  `json:"id"`
	}
	if err := g.doAuthed(token, "GET", "/user", &body); err != nil {
		return "", 0, err
	}
	return body.Login, body.ID, nil
}

func (g *GitHubClient) ListOrgs(token string) ([]string, error) {
	var orgs []struct {
		Login string `json:"login"`
	}
	if err := g.doAuthed(token, "GET", "/user/orgs", &orgs); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(orgs))
	for _, o := range orgs {
		out = append(out, o.Login)
	}
	return out, nil
}
