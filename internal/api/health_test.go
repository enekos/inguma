package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	s := newTestServer(t)
	r := httptest.NewRequest(http.MethodGet, "/api/_health", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("code = %d, body = %s", w.Code, w.Body.String())
	}
	var parsed struct {
		Status      string `json:"status"`
		ToolCount   int    `json:"tool_count"`
		FailedCount int    `json:"failed_count"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.Status != "ok" || parsed.ToolCount != 3 || parsed.FailedCount != 0 {
		t.Errorf("got = %+v", parsed)
	}
}
