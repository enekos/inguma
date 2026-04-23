package namespace

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in          string
		wantErr     bool
		owner, slug string
		isBare      bool
	}{
		{"@foo/bar", false, "foo", "bar", false},
		{"@Foo/Bar", false, "foo", "bar", false},
		{"bar", false, "", "bar", true},
		{"@foo/bar/baz", true, "", "", false},
		{"@/bar", true, "", "", false},
		{"@foo/", true, "", "", false},
		{"@foo/BAR-baz_1", true, "", "", false},
		{"@foo/bar-baz", false, "foo", "bar-baz", false},
	}
	for _, c := range cases {
		n, err := Parse(c.in)
		if (err != nil) != c.wantErr {
			t.Fatalf("%q: wantErr=%v got err=%v", c.in, c.wantErr, err)
		}
		if err != nil {
			continue
		}
		if n.Owner != c.owner || n.Slug != c.slug || n.IsBare != c.isBare {
			t.Fatalf("%q: got %+v", c.in, n)
		}
	}
}

func TestCanonical(t *testing.T) {
	n, _ := Parse("@Foo/Bar-Baz")
	if n.Canonical() != "@foo/bar-baz" {
		t.Fatalf("canonical: %s", n.Canonical())
	}
}
