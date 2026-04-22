// Package marrow is a thin HTTP client for the Marrow search service.
// See ~/marrow/README.md for Marrow's API surface.
package marrow

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Query is the request shape for POST /search.
type Query struct {
	Q     string `json:"q"`
	Limit int    `json:"limit,omitempty"`
	Lang  string `json:"lang,omitempty"`
}

// Result is a single hit from /search.
// Marrow returns more fields; we only decode what we need.
type Result struct {
	Slug  string  `json:"slug"`
	Score float64 `json:"score"`
}

type searchResponse struct {
	Results []Result `json:"results"`
}

// Client is a Marrow HTTP client.
type Client struct {
	baseURL string
	http    *http.Client
}

// New returns a client that talks to the given Marrow base URL (e.g. "http://localhost:8080").
func New(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http:    &http.Client{Timeout: 10 * time.Second},
	}
}

// Search calls POST /search and returns the result list.
func (c *Client) Search(ctx context.Context, q Query) ([]Result, error) {
	body, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("marrow: search: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("marrow: search status %d: %s", resp.StatusCode, b)
	}
	var out searchResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("marrow: decode: %w", err)
	}
	return out.Results, nil
}
