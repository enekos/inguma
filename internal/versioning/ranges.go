package versioning

import (
	"errors"
	"sort"
	"strings"
)

type Range struct {
	kind  rangeKind
	bound Version
}

type rangeKind int

const (
	rangeLatest rangeKind = iota
	rangeExact
	rangeCaret
	rangeTilde
)

func ParseRange(spec string) (Range, error) {
	s := strings.TrimSpace(spec)
	if s == "" || s == "latest" {
		return Range{kind: rangeLatest}, nil
	}
	switch s[0] {
	case '^':
		v, err := normalizeBound(s[1:])
		if err != nil {
			return Range{}, err
		}
		return Range{kind: rangeCaret, bound: v}, nil
	case '~':
		v, err := normalizeBound(s[1:])
		if err != nil {
			return Range{}, err
		}
		return Range{kind: rangeTilde, bound: v}, nil
	}
	v, err := ParseVersion(s)
	if err != nil {
		return Range{}, err
	}
	return Range{kind: rangeExact, bound: v}, nil
}

func normalizeBound(s string) (Version, error) {
	if s == "" {
		return Version{}, errors.New("empty bound")
	}
	parts := strings.SplitN(strings.TrimPrefix(s, "v"), ".", 3)
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	return ParseVersion("v" + strings.Join(parts, "."))
}

func (r Range) Matches(v Version) bool {
	switch r.kind {
	case rangeLatest:
		return !v.IsPrerelease()
	case rangeExact:
		return v.Compare(r.bound) == 0
	case rangeCaret:
		if v.IsPrerelease() {
			return false
		}
		return v.Major() == r.bound.Major() && v.Compare(r.bound) >= 0
	case rangeTilde:
		if v.IsPrerelease() {
			return false
		}
		return v.MajorMinor() == r.bound.MajorMinor() && v.Compare(r.bound) >= 0
	}
	return false
}

func (r Range) HighestMatch(all []Version) (Version, bool) {
	sorted := make([]Version, 0, len(all))
	for _, v := range all {
		if r.Matches(v) {
			sorted = append(sorted, v)
		}
	}
	if len(sorted) == 0 {
		return Version{}, false
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Compare(sorted[j]) < 0 })
	return sorted[len(sorted)-1], true
}
