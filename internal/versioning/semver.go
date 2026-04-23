// Package versioning wraps golang.org/x/mod/semver with inguma-specific
// rules: require full major.minor.patch, normalize to canonical "vX.Y.Z",
// surface prerelease as a first-class property.
package versioning

import (
	"errors"
	"strings"

	"golang.org/x/mod/semver"
)

type Version struct{ s string }

func ParseVersion(in string) (Version, error) {
	if in == "" {
		return Version{}, errors.New("empty version")
	}
	s := in
	if !strings.HasPrefix(s, "v") {
		s = "v" + s
	}
	if !semver.IsValid(s) {
		return Version{}, errors.New("not valid semver: " + in)
	}
	// Reject "v1" or "v1.2" — require full triple.
	if semver.Canonical(s) != s && !strings.Contains(s, "-") && !strings.Contains(s, "+") {
		return Version{}, errors.New("must be major.minor.patch: " + in)
	}
	return Version{s: s}, nil
}

func (v Version) Canonical() string     { return v.s }
func (v Version) IsPrerelease() bool    { return semver.Prerelease(v.s) != "" }
func (v Version) Compare(o Version) int { return semver.Compare(v.s, o.s) }
func (v Version) Major() string         { return semver.Major(v.s) }
func (v Version) MajorMinor() string    { return semver.MajorMinor(v.s) }
func (v Version) String() string        { return v.s }
