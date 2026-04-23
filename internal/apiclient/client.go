// Package apiclient is the inguma CLI's HTTP client for the apid API.
package apiclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/enekos/inguma/internal/manifest"
)

// ToolResponse mirrors GET /api/tools/{slug}.
type ToolResponse struct {
	Slug     string        `json:"slug"`
	Manifest manifest.Tool `json:"manifest"`
	Readme   string        `json:"readme"`
}

// SearchHit mirrors one entry of GET /api/search.
type SearchHit struct {
	Slug  string  `json:"slug"`
	Score float64 `json:"score"`
	Tool  struct {
		DisplayName string   `json:"display_name"`
		Description string   `json:"description"`
		Kind        string   `json:"kind"`
		Categories  []string `json:"categories"`
	} `json:"tool"`
}

// InstallResponse mirrors GET /api/install/{slug}.
type InstallResponse struct {
	Slug string `json:"slug"`
	CLI  struct {
		Command string `json:"command"`
	} `json:"cli"`
	Snippets []struct {
		HarnessID   string `json:"harness_id"`
		DisplayName string `json:"display_name"`
		Format      string `json:"format"`
		Path        string `json:"path"`
		Content     string `json:"content"`
	} `json:"snippets"`
}

// Client talks to an apid instance.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a client rooted at baseURL (e.g. "https://inguma.example").
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

// GetTool fetches a tool's canonical manifest + README.
func (c *Client) GetTool(slug string) (*ToolResponse, error) {
	var out ToolResponse
	if err := c.getJSON("/api/tools/"+url.PathEscape(slug), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetInstall fetches per-harness install snippets + CLI one-liner.
func (c *Client) GetInstall(slug string) (*InstallResponse, error) {
	var out InstallResponse
	if err := c.getJSON("/api/install/"+url.PathEscape(slug), &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// SearchFilters are optional structured filters sent to /api/search.
type SearchFilters struct {
	Kind     string
	Harness  string
	Category string
	Platform string
}

// Search runs a marketplace search and returns hydrated hits.
func (c *Client) Search(q string, f *SearchFilters) ([]SearchHit, error) {
	v := url.Values{}
	v.Set("q", q)
	if f != nil {
		if f.Kind != "" {
			v.Set("kind", f.Kind)
		}
		if f.Harness != "" {
			v.Set("harness", f.Harness)
		}
		if f.Category != "" {
			v.Set("category", f.Category)
		}
		if f.Platform != "" {
			v.Set("platform", f.Platform)
		}
	}
	var out struct {
		Results []SearchHit `json:"results"`
	}
	if err := c.getJSON("/api/search?"+v.Encode(), &out); err != nil {
		return nil, err
	}
	return out.Results, nil
}

// VersionedInstallResponse mirrors GET /api/install/@{owner}/{slug} and .../@{version}.
type VersionedInstallResponse struct {
	Owner           string `json:"owner"`
	Slug            string `json:"slug"`
	ResolvedVersion string `json:"resolved_version"`
	SHA256          string `json:"sha256"`
	CLI             struct {
		Command string `json:"command"`
	} `json:"cli"`
	Snippets []struct {
		HarnessID   string `json:"harness_id"`
		DisplayName string `json:"display_name"`
		Format      string `json:"format"`
		Path        string `json:"path"`
		Content     string `json:"content"`
	} `json:"snippets"`
}

// VersionedToolResponse mirrors GET /api/tools/@{owner}/{slug}[/@version].
type VersionedToolResponse struct {
	Owner         string        `json:"owner"`
	Slug          string        `json:"slug"`
	LatestVersion string        `json:"latest_version"`
	Version       string        `json:"version"`
	Versions      []string      `json:"versions"`
	Manifest      manifest.Tool `json:"manifest"`
	Readme        string        `json:"readme"`
}

// GetVersionedTool fetches /api/tools/@{owner}/{slug} (latest) or /api/tools/@{owner}/{slug}/@{version}.
func (c *Client) GetVersionedTool(owner, slug, version string) (*VersionedToolResponse, error) {
	path := "/api/tools/" + url.PathEscape("@"+owner) + "/" + url.PathEscape(slug)
	if version != "" {
		path += "/" + url.PathEscape("@"+version)
	}
	var out VersionedToolResponse
	if err := c.getJSON(path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetVersionedInstall fetches /api/install/@{owner}/{slug} with optional rangeSpec (e.g. "^1.2") or /api/install/@{owner}/{slug}/@{version}.
// Exactly one of version or rangeSpec may be non-empty; both empty means "latest".
func (c *Client) GetVersionedInstall(owner, slug, version, rangeSpec string) (*VersionedInstallResponse, error) {
	path := "/api/install/" + url.PathEscape("@"+owner) + "/" + url.PathEscape(slug)
	if version != "" {
		path += "/" + url.PathEscape("@"+version)
	} else if rangeSpec != "" {
		path += "?range=" + url.QueryEscape(rangeSpec)
	}
	var out VersionedInstallResponse
	if err := c.getJSON(path, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) getJSON(path string, out any) error {
	resp, err := c.http.Get(c.baseURL + path)
	if err != nil {
		return fmt.Errorf("apiclient: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("apiclient: %s → %d: %s", path, resp.StatusCode, b)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}
