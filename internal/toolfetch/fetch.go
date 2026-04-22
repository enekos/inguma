// Package toolfetch handles the actual fetching and installation of CLI-kind tools:
// picking the first supported install source and running it (npm/go) or
// downloading + verifying a checksummed binary.
package toolfetch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/enekos/agentpop/internal/manifest"
)

// haveFn reports whether a command is available on PATH.
// Extracted for tests.
type haveFn func(cmd string) bool

func realHave(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// pickSource returns the first install source whose tooling is available.
// The caller guarantees kind=cli.
func pickSource(m manifest.Tool, have haveFn) (manifest.InstallSource, bool) {
	for _, s := range m.CLI.Install {
		switch s.Type {
		case "npm":
			if have("npm") {
				return s, true
			}
		case "go":
			if have("go") {
				return s, true
			}
		case "binary":
			return s, true // always possible
		}
	}
	return manifest.InstallSource{}, false
}

// Install picks a source and runs it. For npm/go it shells out; for binary it
// fetches into ~/.agentpop/bin/<bin> (creating the dir). Returns the source
// string to record in state (e.g. "npm:@scope/pkg").
func Install(m manifest.Tool) (source string, err error) {
	return installWith(m, realHave, defaultBinDir())
}

// installWith is the testable seam.
func installWith(m manifest.Tool, have haveFn, binDir string) (string, error) {
	src, ok := pickSource(m, have)
	if !ok {
		return "", fmt.Errorf("toolfetch: no supported install source for %s (tried %d)", m.Name, len(m.CLI.Install))
	}
	switch src.Type {
	case "npm":
		cmd := exec.Command("npm", "install", "-g", src.Package)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("toolfetch: npm install %s: %w", src.Package, err)
		}
		return "npm:" + src.Package, nil
	case "go":
		cmd := exec.Command("go", "install", src.Module+"@latest")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("toolfetch: go install %s: %w", src.Module, err)
		}
		return "go:" + src.Module, nil
	case "binary":
		if err := os.MkdirAll(binDir, 0o755); err != nil {
			return "", err
		}
		dest := filepath.Join(binDir, m.CLI.Bin)
		if err := fetchBinary(src, dest); err != nil {
			return "", err
		}
		return "binary:" + expandTemplate(src.URLTemplate), nil
	default:
		return "", fmt.Errorf("toolfetch: unsupported source type %q", src.Type)
	}
}

// fetchBinary downloads the url_template expansion to dest, verifies sha256 if
// sha256_template is provided, makes the file executable.
func fetchBinary(src manifest.InstallSource, dest string) error {
	binURL := expandTemplate(src.URLTemplate)
	cli := &http.Client{Timeout: 60 * time.Second}
	resp, err := cli.Get(binURL)
	if err != nil {
		return fmt.Errorf("toolfetch: get %s: %w", binURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("toolfetch: %s status %d", binURL, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if src.SHA256Template != "" {
		sumURL := expandTemplate(src.SHA256Template)
		resp2, err := cli.Get(sumURL)
		if err != nil {
			return fmt.Errorf("toolfetch: get %s: %w", sumURL, err)
		}
		defer resp2.Body.Close()
		if resp2.StatusCode >= 300 {
			return fmt.Errorf("toolfetch: %s status %d", sumURL, resp2.StatusCode)
		}
		sumRaw, err := io.ReadAll(resp2.Body)
		if err != nil {
			return err
		}
		wantHex := strings.TrimSpace(strings.Split(string(sumRaw), " ")[0])
		gotSum := sha256.Sum256(data)
		gotHex := hex.EncodeToString(gotSum[:])
		if gotHex != wantHex {
			return fmt.Errorf("toolfetch: checksum mismatch for %s: got %s want %s", binURL, gotHex, wantHex)
		}
	}

	if err := os.WriteFile(dest, data, 0o755); err != nil {
		return err
	}
	return nil
}

// expandTemplate substitutes {os} and {arch} from runtime.GOOS / runtime.GOARCH.
func expandTemplate(t string) string {
	t = strings.ReplaceAll(t, "{os}", runtime.GOOS)
	t = strings.ReplaceAll(t, "{arch}", runtime.GOARCH)
	return t
}

func defaultBinDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agentpop", "bin")
}
