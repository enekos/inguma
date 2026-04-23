package clicmd

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enekos/inguma/internal/apiclient"
)

// minimalManifest returns a minimal valid inguma.yaml content with @owner/slug name and version.
func minimalManifest(name, version string) string {
	v := ""
	if version != "" {
		v = "\nversion: " + version
	}
	// Name is quoted to handle '@' characters which YAML would otherwise reject.
	return `name: "` + name + `"` + v + `
display_name: Test Tool
description: A test tool.
readme: README.md
license: MIT
kind: mcp
mcp:
  transport: stdio
  command: test-tool
compatibility:
  harnesses: []
  platforms: []
`
}

// fakeGitRecorder records git calls and returns configured responses.
type fakeGitRecorder struct {
	calls    [][]string
	handlers map[string]func(args []string) ([]byte, error)
}

func (f *fakeGitRecorder) Run(dir string, args ...string) ([]byte, error) {
	key := args[0]
	f.calls = append(f.calls, append([]string{}, args...))
	if h, ok := f.handlers[key]; ok {
		return h(args)
	}
	return nil, nil
}

func (f *fakeGitRecorder) gitFunc() func(dir string, args ...string) ([]byte, error) {
	return f.Run
}

// noopSleep is a sleep func that does nothing, making tests instant.
func noopSleep(_ time.Duration) {}

// frozenNow returns a fixed time.Now func whose deadline won't expire during fast tests.
func frozenNow() func() time.Time {
	t := time.Now().Add(10 * time.Minute)
	return func() time.Time { return t }
}

func TestPublishTagsPushesAndPolls(t *testing.T) {
	// Set up a fake HTTP server that returns 404 twice then 200.
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount <= 2 {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"owner":"foo","slug":"bar","version":"v1.2.3","latest_version":"v1.2.3","versions":["v1.2.3"],"manifest":{},"readme":""}`))
	}))
	defer srv.Close()

	// Write inguma.yaml to a temp dir.
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "inguma.yaml"), []byte(minimalManifest("@foo/bar", "1.2.3")), 0644); err != nil {
		t.Fatal(err)
	}

	// Set up fake git.
	rec := &fakeGitRecorder{
		handlers: map[string]func(args []string) ([]byte, error){
			"status": func(args []string) ([]byte, error) {
				// clean tree
				return []byte{}, nil
			},
			"rev-parse": func(args []string) ([]byte, error) {
				// tag doesn't exist — return error (non-zero exit)
				return nil, errors.New("exit status 128")
			},
			"tag": func(args []string) ([]byte, error) {
				return []byte{}, nil
			},
			"push": func(args []string) ([]byte, error) {
				return []byte{}, nil
			},
		},
	}

	var out bytes.Buffer
	err := Publish(context.Background(), PublishDeps{
		API:    apiclient.New(srv.URL),
		Stdout: &out,
		Git:    rec.gitFunc(),
		Sleep:  noopSleep,
		Now:    frozenNow(),
	}, PublishArgs{
		RepoDir: dir,
		Remote:  "origin",
		Timeout: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Publish returned error: %v", err)
	}

	// Assert 4 git commands ran in order.
	wantCmds := []string{"status", "rev-parse", "tag", "push"}
	if len(rec.calls) != len(wantCmds) {
		t.Fatalf("expected %d git calls, got %d: %v", len(wantCmds), len(rec.calls), rec.calls)
	}
	for i, want := range wantCmds {
		if rec.calls[i][0] != want {
			t.Errorf("git call[%d]: want %q, got %q", i, want, rec.calls[i][0])
		}
	}

	// Assert tag and push used v1.2.3.
	if rec.calls[2][1] != "v1.2.3" {
		t.Errorf("tag call: want v1.2.3, got %q", rec.calls[2][1])
	}
	if rec.calls[3][1] != "origin" || rec.calls[3][2] != "v1.2.3" {
		t.Errorf("push call: want [origin v1.2.3], got %v", rec.calls[3][1:])
	}

	// Poll succeeded — output should mention ingested.
	if !strings.Contains(out.String(), "ingested") {
		t.Errorf("expected 'ingested' in output, got: %q", out.String())
	}

	// API was called 3 times (404, 404, 200).
	if callCount != 3 {
		t.Errorf("expected 3 API calls, got %d", callCount)
	}
}

func TestPublishDirtyTree(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "inguma.yaml"), []byte(minimalManifest("@foo/bar", "1.0.0")), 0644); err != nil {
		t.Fatal(err)
	}

	rec := &fakeGitRecorder{
		handlers: map[string]func(args []string) ([]byte, error){
			"status": func(args []string) ([]byte, error) {
				return []byte("M foo.txt\n"), nil
			},
		},
	}

	err := Publish(context.Background(), PublishDeps{
		API:    apiclient.New("http://localhost:0"),
		Stdout: &bytes.Buffer{},
		Git:    rec.gitFunc(),
		Sleep:  noopSleep,
		Now:    frozenNow(),
	}, PublishArgs{RepoDir: dir})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "dirty") {
		t.Errorf("expected error to mention 'dirty', got: %v", err)
	}
}

func TestPublishTagExists(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "inguma.yaml"), []byte(minimalManifest("@foo/bar", "2.0.0")), 0644); err != nil {
		t.Fatal(err)
	}

	rec := &fakeGitRecorder{
		handlers: map[string]func(args []string) ([]byte, error){
			"status": func(args []string) ([]byte, error) {
				return []byte{}, nil
			},
			"rev-parse": func(args []string) ([]byte, error) {
				// tag exists — return a sha
				return []byte("abc123def456\n"), nil
			},
		},
	}

	err := Publish(context.Background(), PublishDeps{
		API:    apiclient.New("http://localhost:0"),
		Stdout: &bytes.Buffer{},
		Git:    rec.gitFunc(),
		Sleep:  noopSleep,
		Now:    frozenNow(),
	}, PublishArgs{RepoDir: dir})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("expected error to mention 'already exists', got: %v", err)
	}
}

func TestPublishInvalidName(t *testing.T) {
	dir := t.TempDir()
	// bare name (no @owner/)
	if err := os.WriteFile(filepath.Join(dir, "inguma.yaml"), []byte(minimalManifest("bar", "1.0.0")), 0644); err != nil {
		t.Fatal(err)
	}

	err := Publish(context.Background(), PublishDeps{
		API:    apiclient.New("http://localhost:0"),
		Stdout: &bytes.Buffer{},
		Git:    func(dir string, args ...string) ([]byte, error) { return nil, nil },
		Sleep:  noopSleep,
		Now:    frozenNow(),
	}, PublishArgs{RepoDir: dir})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "@owner/slug") {
		t.Errorf("expected error to mention '@owner/slug', got: %v", err)
	}
}

func TestPublishMissingVersion(t *testing.T) {
	dir := t.TempDir()
	// manifest without version field
	if err := os.WriteFile(filepath.Join(dir, "inguma.yaml"), []byte(minimalManifest("@foo/bar", "")), 0644); err != nil {
		t.Fatal(err)
	}

	err := Publish(context.Background(), PublishDeps{
		API:    apiclient.New("http://localhost:0"),
		Stdout: &bytes.Buffer{},
		Git:    func(dir string, args ...string) ([]byte, error) { return nil, nil },
		Sleep:  noopSleep,
		Now:    frozenNow(),
	}, PublishArgs{RepoDir: dir})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "must declare top-level version") {
		t.Errorf("expected error to mention 'must declare top-level version', got: %v", err)
	}
}
