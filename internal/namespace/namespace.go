// Package namespace parses and canonicalizes @owner/slug identifiers.
package namespace

import (
	"errors"
	"regexp"
	"strings"
)

// Name is the canonical form of a package identifier.
// IsBare=true means the caller passed a legacy bare slug with no owner,
// and Owner is empty.
type Name struct {
	Owner  string
	Slug   string
	IsBare bool
}

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)
var ownerRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

func Parse(in string) (Name, error) {
	s := strings.ToLower(strings.TrimSpace(in))
	if s == "" {
		return Name{}, errors.New("empty name")
	}
	if !strings.HasPrefix(s, "@") {
		if !slugRe.MatchString(s) {
			return Name{}, errors.New("invalid bare slug: " + in)
		}
		return Name{Slug: s, IsBare: true}, nil
	}
	rest := s[1:]
	parts := strings.Split(rest, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return Name{}, errors.New("expected @owner/slug: " + in)
	}
	if !ownerRe.MatchString(parts[0]) {
		return Name{}, errors.New("invalid owner: " + parts[0])
	}
	if !slugRe.MatchString(parts[1]) {
		return Name{}, errors.New("invalid slug: " + parts[1])
	}
	return Name{Owner: parts[0], Slug: parts[1]}, nil
}

func (n Name) Canonical() string {
	if n.IsBare {
		return n.Slug
	}
	return "@" + n.Owner + "/" + n.Slug
}
