package versioning

import "testing"

func mustV(t *testing.T, s string) Version {
	t.Helper()
	v, err := ParseVersion(s)
	if err != nil {
		t.Fatalf("bad version %q: %v", s, err)
	}
	return v
}

func TestRangeSelect(t *testing.T) {
	all := []Version{
		mustV(t, "v1.0.0"),
		mustV(t, "v1.2.3"),
		mustV(t, "v1.2.4"),
		mustV(t, "v1.3.0"),
		mustV(t, "v2.0.0"),
		mustV(t, "v2.0.0-beta.1"),
	}
	cases := []struct {
		spec string
		want string
	}{
		{"", "v2.0.0"},
		{"latest", "v2.0.0"},
		{"1.2.3", "v1.2.3"},
		{"^1.2", "v1.3.0"},
		{"^1.2.4", "v1.3.0"},
		{"~1.2", "v1.2.4"},
	}
	for _, c := range cases {
		r, err := ParseRange(c.spec)
		if err != nil {
			t.Fatalf("spec=%q: ParseRange err: %v", c.spec, err)
		}
		got, ok := r.HighestMatch(all)
		if !ok {
			t.Fatalf("spec=%q: no match", c.spec)
		}
		if got.Canonical() != c.want {
			t.Fatalf("spec=%q: got %s want %s", c.spec, got.Canonical(), c.want)
		}
	}
}

func TestRangeNoPrerelease(t *testing.T) {
	all := []Version{mustV(t, "v1.0.0-beta.1"), mustV(t, "v0.9.0")}
	r, _ := ParseRange("")
	got, _ := r.HighestMatch(all)
	if got.Canonical() != "v0.9.0" {
		t.Fatalf("expected v0.9.0, got %s", got.Canonical())
	}
}

func TestRangePrereleaseExplicit(t *testing.T) {
	all := []Version{mustV(t, "v1.0.0-beta.1")}
	r, _ := ParseRange("1.0.0-beta.1")
	got, ok := r.HighestMatch(all)
	if !ok || got.Canonical() != "v1.0.0-beta.1" {
		t.Fatalf("expected exact prerelease match")
	}
}
