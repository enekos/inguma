// Package sigstore is a thin wrapper around cosign keyless verification.
//
// The crawler shells out to `cosign verify-blob` if it's on PATH; when
// cosign is absent or verification fails, the caller records the
// artifact as unsigned. Tests can substitute a Verifier to avoid the
// external binary.
package sigstore

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Identity is the subject we require to match in the certificate.
// Both fields use cosign's --certificate-identity-regexp / --certificate-oidc-issuer semantics.
type Identity struct {
	// IdentityRegexp matches the workflow URL, e.g.
	//   ^https://github.com/foo/bar/.github/workflows/.+@refs/tags/v.*$
	IdentityRegexp string
	// OIDCIssuer pins the issuer URL (default: https://token.actions.githubusercontent.com).
	OIDCIssuer string
}

// Result is what the crawler stores in the signatures table.
type Result struct {
	Verified     bool
	CertIdentity string
	CertIssuer   string
	VerifiedAt   time.Time
	Reason       string // non-empty on failure
}

// Verifier is the seam the crawler and tests use.
type Verifier interface {
	Verify(sha256hex string, bundlePath string, id Identity) Result
}

// Cosign is the production verifier. It requires `cosign` on PATH.
type Cosign struct {
	Bin string // default: cosign
}

func (c *Cosign) binary() string {
	if c == nil || c.Bin == "" {
		return "cosign"
	}
	return c.Bin
}

// Verify runs `cosign verify-blob --bundle <bundle> --certificate-identity-regexp ... --certificate-oidc-issuer ...`.
// The digest is passed via stdin to keep the tarball off disk.
func (c *Cosign) Verify(sha256hex, bundlePath string, id Identity) Result {
	if _, err := exec.LookPath(c.binary()); err != nil {
		return Result{Reason: "cosign not on PATH"}
	}
	if bundlePath == "" {
		return Result{Reason: "no sigstore bundle supplied"}
	}
	issuer := id.OIDCIssuer
	if issuer == "" {
		issuer = "https://token.actions.githubusercontent.com"
	}
	args := []string{
		"verify-blob",
		"--bundle", bundlePath,
		"--certificate-identity-regexp", id.IdentityRegexp,
		"--certificate-oidc-issuer", issuer,
		"-",
	}
	cmd := exec.Command(c.binary(), args...)
	cmd.Stdin = strings.NewReader(sha256hex)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return Result{Reason: fmt.Sprintf("cosign verify-blob: %v: %s", err, strings.TrimSpace(stderr.String()))}
	}
	return Result{
		Verified:     true,
		CertIdentity: id.IdentityRegexp,
		CertIssuer:   issuer,
		VerifiedAt:   time.Now().UTC(),
	}
}

// IdentityForRepo computes the workflow-identity regexp we require for
// an artifact that claims to come from github.com/<owner>/<slug>.
func IdentityForRepo(owner, slug string) Identity {
	return Identity{
		IdentityRegexp: fmt.Sprintf(`^https://github\.com/%s/%s/\.github/workflows/.+$`, regexpQuote(owner), regexpQuote(slug)),
		OIDCIssuer:     "https://token.actions.githubusercontent.com",
	}
}

func regexpQuote(s string) string {
	// cosign uses RE2. Only '.' and '-' typically appear in GH logins.
	r := strings.NewReplacer(".", `\.`, "+", `\+`, "*", `\*`)
	return r.Replace(s)
}

// ErrNotSigned is returned by higher-level helpers when an artifact has
// no associated signature bundle.
var ErrNotSigned = errors.New("not signed")
