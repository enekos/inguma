package versioning

import "testing"

func TestParseVersion(t *testing.T) {
	cases := []struct {
		in      string
		wantErr bool
		canon   string
	}{
		{"v1.2.3", false, "v1.2.3"},
		{"1.2.3", false, "v1.2.3"},
		{"v1.2.3-beta.1", false, "v1.2.3-beta.1"},
		{"v1.2", true, ""},
		{"latest", true, ""},
		{"", true, ""},
	}
	for _, c := range cases {
		got, err := ParseVersion(c.in)
		if (err != nil) != c.wantErr {
			t.Fatalf("%q: wantErr=%v got err=%v", c.in, c.wantErr, err)
		}
		if err == nil && got.Canonical() != c.canon {
			t.Fatalf("%q: canon=%q want %q", c.in, got.Canonical(), c.canon)
		}
	}
}

func TestCompare(t *testing.T) {
	a, _ := ParseVersion("v1.2.3")
	b, _ := ParseVersion("v1.2.4")
	if a.Compare(b) >= 0 {
		t.Fatal("expected a < b")
	}
	pre, _ := ParseVersion("v1.2.3-beta.1")
	if pre.Compare(a) >= 0 {
		t.Fatal("expected prerelease < release")
	}
}

func TestIsPrerelease(t *testing.T) {
	v, _ := ParseVersion("v1.2.3-beta.1")
	if !v.IsPrerelease() {
		t.Fatal("expected prerelease")
	}
	v2, _ := ParseVersion("v1.2.3")
	if v2.IsPrerelease() {
		t.Fatal("expected stable")
	}
}
