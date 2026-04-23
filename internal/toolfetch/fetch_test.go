package toolfetch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enekos/inguma/internal/manifest"
)

func TestPickSource_prefersFirstAvailable(t *testing.T) {
	m := manifest.Tool{
		Kind: manifest.KindCLI,
		CLI: &manifest.CLIConfig{
			Install: []manifest.InstallSource{
				{Type: "npm", Package: "@x/y"},
				{Type: "binary", URLTemplate: "https://example.com/bin"},
			},
		},
	}

	// Fake "which" that pretends only `curl` is on PATH (i.e. npm is not).
	have := func(cmd string) bool { return cmd == "curl" }
	src, ok := pickSource(m, have)
	if !ok {
		t.Fatal("want ok")
	}
	if src.Type != "binary" {
		t.Errorf("picked %q, want binary", src.Type)
	}

	// Now npm is available: should pick the first matching source.
	have2 := func(cmd string) bool { return cmd == "npm" || cmd == "curl" }
	src, ok = pickSource(m, have2)
	if !ok || src.Type != "npm" {
		t.Errorf("got %+v, ok=%v", src, ok)
	}
}

func TestFetchBinary_verifiesChecksum(t *testing.T) {
	payload := []byte("#!/bin/sh\necho hi\n")
	sum := sha256.Sum256(payload)
	sumHex := hex.EncodeToString(sum[:])

	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if strings.HasSuffix(r.URL.Path, ".sha256") {
			fmt.Fprintln(w, sumHex+"  tool")
			return
		}
		w.Write(payload)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "tool")
	src := manifest.InstallSource{
		Type:           "binary",
		URLTemplate:    srv.URL + "/tool-{os}-{arch}",
		SHA256Template: srv.URL + "/tool-{os}-{arch}.sha256",
	}
	if err := fetchBinary(src, dest); err != nil {
		t.Fatalf("fetchBinary: %v", err)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != string(payload) {
		t.Errorf("payload mismatch")
	}
	info, _ := os.Stat(dest)
	if info.Mode()&0o100 == 0 {
		t.Errorf("binary not executable: %v", info.Mode())
	}
}

func TestFetchBinary_rejectsBadChecksum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".sha256") {
			fmt.Fprintln(w, "dead"+strings.Repeat("b", 60)+"  tool")
			return
		}
		w.Write([]byte("payload"))
	}))
	defer srv.Close()

	err := fetchBinary(manifest.InstallSource{
		Type:           "binary",
		URLTemplate:    srv.URL + "/tool-{os}-{arch}",
		SHA256Template: srv.URL + "/tool-{os}-{arch}.sha256",
	}, filepath.Join(t.TempDir(), "tool"))
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("want checksum error, got %v", err)
	}
}
