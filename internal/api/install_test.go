package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestInstall_returnsSnippetPerAdapter(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/install/tool-a", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var parsed struct {
		Slug string `json:"slug"`
		CLI  struct {
			Command string `json:"command"`
		} `json:"cli"`
		Snippets []struct {
			HarnessID   string `json:"harness_id"`
			DisplayName string `json:"display_name"`
			Format      string `json:"format"`
			Content     string `json:"content"`
		} `json:"snippets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Slug != "tool-a" {
		t.Errorf("slug = %q", parsed.Slug)
	}
	if parsed.CLI.Command != "inguma install tool-a" {
		t.Errorf("cli.command = %q", parsed.CLI.Command)
	}
	// all.Default() registers claude-code + cursor
	if len(parsed.Snippets) != 2 {
		t.Fatalf("snippets len = %d: %+v", len(parsed.Snippets), parsed.Snippets)
	}
	for _, sn := range parsed.Snippets {
		if sn.Content == "" {
			t.Errorf("empty content for %s", sn.HarnessID)
		}
	}
}

func TestInstall_notFound(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/install/no-such-tool", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Errorf("code = %d", w.Code)
	}
}
