package versioning

import "sort"

// ScanTags filters raw git tag names down to those that parse as a strict
// major.minor.patch[+prerelease] and returns them sorted ascending.
func ScanTags(tags []string) []Version {
	out := make([]Version, 0, len(tags))
	for _, t := range tags {
		v, err := ParseVersion(t)
		if err != nil {
			continue
		}
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Compare(out[j]) < 0 })
	return out
}
