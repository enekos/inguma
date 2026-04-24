package bundles

import "testing"

import "github.com/enekos/inguma/internal/manifest"

func TestExpand_Basic(t *testing.T) {
	b := &manifest.BundleConfig{
		Includes: []string{"@foo/mcp-github@^1", "@bar/skill-testing@v2.0.1", "@baz/subagent-reviewer"},
		Defaults: map[string]manifest.Member{
			"@foo/mcp-github": {Env: map[string]string{"GITHUB_TOKEN": "$GITHUB_TOKEN"}},
		},
	}
	out, err := Expand(b, "@team/workflow")
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 3 {
		t.Fatalf("want 3 members, got %d", len(out))
	}
	if out[0].Range != "^1" {
		t.Fatalf("range=%q", out[0].Range)
	}
	if out[1].Version != "v2.0.1" {
		t.Fatalf("version=%q", out[1].Version)
	}
	if out[2].Range != "" || out[2].Version != "" {
		t.Fatalf("unpinned member should have empty range+version")
	}
	if out[0].Env["GITHUB_TOKEN"] != "$GITHUB_TOKEN" {
		t.Fatalf("env defaults not applied: %+v", out[0].Env)
	}
}

func TestExpand_SelfInclude(t *testing.T) {
	b := &manifest.BundleConfig{Includes: []string{"@team/workflow"}}
	if _, err := Expand(b, "@team/workflow"); err == nil {
		t.Fatal("want self-include error")
	}
}

func TestExpand_Duplicate(t *testing.T) {
	b := &manifest.BundleConfig{Includes: []string{"@foo/bar", "@foo/bar@v1.0.0"}}
	if _, err := Expand(b, "@team/workflow"); err == nil {
		t.Fatal("want duplicate error")
	}
}

func TestExpand_BareSlugRejected(t *testing.T) {
	b := &manifest.BundleConfig{Includes: []string{"plain-slug"}}
	if _, err := Expand(b, "@team/workflow"); err == nil {
		t.Fatal("want bare-slug error")
	}
}

func TestResolveConflicts(t *testing.T) {
	m := map[string][]Member{
		"a": {{Owner: "foo", Slug: "bar", Version: "v1.0.0"}},
		"b": {{Owner: "foo", Slug: "bar", Version: "v2.0.0"}},
	}
	slug, va, vb := ResolveConflicts(m)
	if slug != "@foo/bar" || va == vb {
		t.Fatalf("want conflict, got slug=%q a=%q b=%q", slug, va, vb)
	}
}
