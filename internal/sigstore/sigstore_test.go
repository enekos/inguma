package sigstore

import (
	"regexp"
	"testing"
)

func TestIdentityForRepoRegexpValid(t *testing.T) {
	id := IdentityForRepo("foo", "bar")
	re, err := regexp.Compile(id.IdentityRegexp)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !re.MatchString("https://github.com/foo/bar/.github/workflows/release.yml") {
		t.Fatalf("regexp %q should match canonical workflow path", id.IdentityRegexp)
	}
	if re.MatchString("https://github.com/attacker/bar/.github/workflows/release.yml") {
		t.Fatalf("regexp matched wrong owner")
	}
}

func TestCosignNoBinary(t *testing.T) {
	// Use an impossible binary name; should return non-verified with a reason.
	r := (&Cosign{Bin: "definitely-not-cosign-xyz"}).Verify("abc", "/tmp/missing.bundle", IdentityForRepo("foo", "bar"))
	if r.Verified {
		t.Fatalf("unexpected verified=true")
	}
	if r.Reason == "" {
		t.Fatalf("expected reason")
	}
}
