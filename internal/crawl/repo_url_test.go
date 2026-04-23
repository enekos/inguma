package crawl

import "testing"

func TestOwnerSlugFromRepo(t *testing.T) {
	cases := []struct {
		repo      string
		owner     string
		slug      string
		wantError bool
	}{
		{
			repo:  "https://github.com/foo/bar",
			owner: "foo",
			slug:  "bar",
		},
		{
			repo:  "https://github.com/foo/bar.git",
			owner: "foo",
			slug:  "bar",
		},
		{
			repo:  "github.com/foo/bar",
			owner: "foo",
			slug:  "bar",
		},
		{
			repo:      "not-a-url",
			wantError: true,
		},
	}
	for _, tc := range cases {
		owner, slug, err := OwnerSlugFromRepo(tc.repo)
		if tc.wantError {
			if err == nil {
				t.Errorf("OwnerSlugFromRepo(%q): want error, got owner=%q slug=%q", tc.repo, owner, slug)
			}
			continue
		}
		if err != nil {
			t.Errorf("OwnerSlugFromRepo(%q): unexpected error: %v", tc.repo, err)
			continue
		}
		if owner != tc.owner || slug != tc.slug {
			t.Errorf("OwnerSlugFromRepo(%q) = (%q, %q), want (%q, %q)", tc.repo, owner, slug, tc.owner, tc.slug)
		}
	}
}
