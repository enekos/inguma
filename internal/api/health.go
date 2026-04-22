package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

type healthResponse struct {
	Status      string `json:"status"`
	ToolCount   int    `json:"tool_count"`
	FailedCount int    `json:"failed_count"`
}

// handleHealth reports whether the corpus is readable and how many tools
// are indexed / failed in the last crawl. Used by the monitoring stack
// to notice stale or rotting corpora.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Read _index.json for the tool count.
	idxData, err := os.ReadFile(filepath.Join(s.CorpusDir, "_index.json"))
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, "corpus_unreadable", "corpus _index.json unavailable")
		return
	}
	var idx struct {
		Tools []struct{} `json:"tools"`
	}
	if err := json.Unmarshal(idxData, &idx); err != nil {
		writeError(w, http.StatusServiceUnavailable, "corpus_unreadable", "corpus _index.json malformed")
		return
	}
	// Read _crawl.json for the failure count (best-effort — missing file is OK).
	var crawl struct {
		Failed []struct{} `json:"failed"`
	}
	if cs, err := os.ReadFile(filepath.Join(s.CorpusDir, "_crawl.json")); err == nil {
		_ = json.Unmarshal(cs, &crawl)
	}
	writeJSON(w, http.StatusOK, healthResponse{
		Status:      "ok",
		ToolCount:   len(idx.Tools),
		FailedCount: len(crawl.Failed),
	})
}
