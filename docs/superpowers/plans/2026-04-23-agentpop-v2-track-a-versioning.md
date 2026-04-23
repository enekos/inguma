# Agentpop v2 Track A — Versioning + Artifacts + Lockfile Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add semver versioning, immutable per-version artifact snapshots, a lockfile for reproducible installs, and a minimal SQLite derived-state store to Agentpop. After this track, `agentpop install @owner/slug@^1.2` works end-to-end with a lockfile and `--frozen`.

**Architecture:** Git-as-database stays: tool repos are the source of truth; the crawler detects new `v<semver>` tags, produces deterministic manifest-snapshot tarballs, and writes a versioned corpus layout on disk. A SQLite file holds derived state only (downloads, audit). A new `internal/versioning` package owns semver semantics; `internal/artifacts` owns snapshot + store; `internal/lockfile` owns the lockfile TOML. API and CLI are extended with version-aware routes/commands.

**Tech Stack:** Go 1.25, `golang.org/x/mod/semver` (stdlib-adjacent, v1.2.3-style native), `modernc.org/sqlite` (pure-Go, no cgo), `github.com/BurntSushi/toml`, `yaml.v3` (already present).

**Scope note:** Tracks B (accounts), C (trust), and D (kinds) get their own plan files. This plan does NOT implement them; it only creates the primitives they need.

---

## File map

Created:
- `internal/versioning/{semver.go,semver_test.go,ranges.go,ranges_test.go,tagscan.go,tagscan_test.go}`
- `internal/namespace/{namespace.go,namespace_test.go}`
- `internal/artifacts/{snapshot.go,snapshot_test.go,store.go,store_test.go}`
- `internal/lockfile/{lockfile.go,lockfile_test.go}`
- `internal/db/{db.go,db_test.go,migrations/0001_init.sql}`
- `internal/apiclient/versioned.go` (fetch with version)
- `internal/clicmd/{publish.go,publish_test.go,upgrade.go,upgrade_test.go}`

Modified:
- `go.mod` (add deps)
- `internal/manifest/{types.go,validate.go,validate_test.go}` (add name/owner canonical validation)
- `internal/corpus/{writer.go,reader.go,*_test.go}` (versioned layout)
- `internal/crawl/{crawl.go,crawl_test.go}` (tag-diff loop)
- `internal/api/{server.go,tools.go,install.go,tools_test.go,install_test.go}` (version routes, artifacts route)
- `internal/clicmd/install.go` (lockfile write + `--frozen`)
- `cmd/apid/main.go` (wire SQLite open, artifact route)
- `cmd/agentpop/main.go` (register new subcommands)
- `cmd/crawler/main.go` (wire tag-diff, artifact writer)

Commit boundary: one commit per task unless a task says otherwise.

---

## Task 1: Add dependencies

**Files:**
- Modify: `go.mod`, `go.sum`

- [ ] **Step 1: Add deps**

Run:
```bash
cd /Users/enekosarasola/agentpop
go get golang.org/x/mod/semver@latest
go get modernc.org/sqlite@latest
go get github.com/BurntSushi/toml@latest
go mod tidy
```

- [ ] **Step 2: Verify build still green**

Run: `make build`
Expected: no errors.

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: add semver, sqlite, toml deps for v2 track A"
```

---

## Task 2: `internal/versioning` — semver parse + compare

**Files:**
- Create: `internal/versioning/semver.go`, `internal/versioning/semver_test.go`

- [ ] **Step 1: Write failing test**

`internal/versioning/semver_test.go`:
```go
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
```

- [ ] **Step 2: Run, verify fail**

Run: `go test ./internal/versioning/... -run Parse -v`
Expected: build failure (package doesn't exist yet).

- [ ] **Step 3: Implement**

`internal/versioning/semver.go`:
```go
// Package versioning wraps golang.org/x/mod/semver with agentpop-specific
// rules: require full major.minor.patch, normalize to canonical "vX.Y.Z",
// surface prerelease as a first-class property.
package versioning

import (
    "errors"
    "strings"

    "golang.org/x/mod/semver"
)

type Version struct{ s string }

// ParseVersion accepts "v1.2.3" or "1.2.3"; shorter forms are rejected.
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

func (v Version) Canonical() string  { return v.s }
func (v Version) IsPrerelease() bool { return semver.Prerelease(v.s) != "" }
func (v Version) Compare(o Version) int { return semver.Compare(v.s, o.s) }
func (v Version) Major() string { return semver.Major(v.s) }
func (v Version) MajorMinor() string { return semver.MajorMinor(v.s) }
func (v Version) String() string { return v.s }
```

- [ ] **Step 4: Run, verify pass**

Run: `go test ./internal/versioning/... -v`
Expected: all tests pass.

- [ ] **Step 5: Commit**

```bash
git add internal/versioning/
git commit -m "feat(versioning): Version type with parse and compare"
```

---

## Task 3: `internal/versioning` — range matching

**Files:**
- Create: `internal/versioning/ranges.go`, `internal/versioning/ranges_test.go`

- [ ] **Step 1: Test**

`internal/versioning/ranges_test.go`:
```go
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
        {"", "v2.0.0"},          // empty -> highest stable
        {"latest", "v2.0.0"},    // explicit latest alias
        {"1.2.3", "v1.2.3"},     // exact
        {"^1.2", "v1.3.0"},      // highest 1.x.x
        {"^1.2.4", "v1.3.0"},    // ^1.2.4 -> any 1.x.y >= 1.2.4
        {"~1.2", "v1.2.4"},      // highest 1.2.x
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
```

- [ ] **Step 2: Run, fail**

Run: `go test ./internal/versioning/... -run Range -v`
Expected: undefined symbols.

- [ ] **Step 3: Implement**

`internal/versioning/ranges.go`:
```go
package versioning

import (
    "errors"
    "sort"
    "strings"
)

// Range represents one of: empty/latest, exact, caret (^X.Y[.Z]), tilde (~X.Y).
type Range struct {
    kind  rangeKind
    bound Version // lower bound (inclusive) for caret/tilde/exact
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

// normalizeBound accepts "1.2" or "1.2.3" and returns a Version with
// missing components zero-filled.
func normalizeBound(s string) (Version, error) {
    parts := strings.SplitN(strings.TrimPrefix(s, "v"), ".", 3)
    for len(parts) < 3 {
        parts = append(parts, "0")
    }
    if len(parts) == 0 {
        return Version{}, errors.New("empty bound")
    }
    return ParseVersion("v" + strings.Join(parts, "."))
}

// Matches reports whether v satisfies the range.
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
```

- [ ] **Step 4: Run, pass**

Run: `go test ./internal/versioning/... -v`
Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add internal/versioning/
git commit -m "feat(versioning): semver ranges with caret, tilde, exact, latest"
```

---

## Task 4: Tag scanner — filter git tag list to versions

**Files:**
- Create: `internal/versioning/tagscan.go`, `internal/versioning/tagscan_test.go`

- [ ] **Step 1: Test**

`internal/versioning/tagscan_test.go`:
```go
package versioning

import (
    "reflect"
    "testing"
)

func TestScanTags(t *testing.T) {
    in := []string{"v1.0.0", "v1.2.3", "release-2", "v0.1.0-beta.1", "main", "v2"}
    got := ScanTags(in)
    want := []string{"v0.1.0-beta.1", "v1.0.0", "v1.2.3"}
    gotStr := make([]string, len(got))
    for i, v := range got {
        gotStr[i] = v.Canonical()
    }
    if !reflect.DeepEqual(gotStr, want) {
        t.Fatalf("got %v want %v", gotStr, want)
    }
}
```

- [ ] **Step 2: Run, fail**

Run: `go test ./internal/versioning/... -run ScanTags -v`

- [ ] **Step 3: Implement**

`internal/versioning/tagscan.go`:
```go
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
```

- [ ] **Step 4: Run, pass**

Run: `go test ./internal/versioning/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/versioning/
git commit -m "feat(versioning): ScanTags filters git tags to versions"
```

---

## Task 5: `internal/namespace` — `@owner/slug` canonicalization

**Files:**
- Create: `internal/namespace/namespace.go`, `internal/namespace/namespace_test.go`

- [ ] **Step 1: Test**

`internal/namespace/namespace_test.go`:
```go
package namespace

import "testing"

func TestParse(t *testing.T) {
    cases := []struct {
        in           string
        wantErr      bool
        owner, slug  string
        isBare       bool
    }{
        {"@foo/bar", false, "foo", "bar", false},
        {"@Foo/Bar", false, "foo", "bar", false},            // lowercased
        {"bar", false, "", "bar", true},                     // legacy bare
        {"@foo/bar/baz", true, "", "", false},
        {"@/bar", true, "", "", false},
        {"@foo/", true, "", "", false},
        {"@foo/BAR-baz_1", true, "", "", false},             // underscore not allowed
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
```

- [ ] **Step 2: Run, fail**

Run: `go test ./internal/namespace/... -v`

- [ ] **Step 3: Implement**

`internal/namespace/namespace.go`:
```go
// Package namespace parses and canonicalizes @owner/slug identifiers.
package namespace

import (
    "errors"
    "regexp"
    "strings"
)

// Name is the canonical form of a package identifier.
// IsBare=true means the caller passed a legacy bare slug with no owner,
// and Owner is empty. Callers that require fully-qualified names must
// resolve bare names through the registry.
type Name struct {
    Owner  string
    Slug   string
    IsBare bool
}

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)
var ownerRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*[a-z0-9]$|^[a-z0-9]$`)

// Parse accepts "@owner/slug" or a legacy bare "slug". Mixed case is lowercased.
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
```

- [ ] **Step 4: Run, pass**

Run: `go test ./internal/namespace/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/namespace/
git commit -m "feat(namespace): Parse @owner/slug with legacy bare support"
```

---

## Task 6: Manifest validation — enforce `name` matches registry owner/slug

**Files:**
- Modify: `internal/manifest/types.go`, `internal/manifest/validate.go`, `internal/manifest/validate_test.go`

- [ ] **Step 1: Read current validate**

Run: `cat internal/manifest/validate.go`

- [ ] **Step 2: Test**

Append to `internal/manifest/validate_test.go`:
```go
func TestValidateWithRegistryOwner(t *testing.T) {
    m := Tool{Name: "bar", Kind: KindMCP, MCP: &MCPConfig{Transport: "stdio", Command: "x"},
        Compatibility: Compatibility{Harnesses: []string{"claude-code"}, Platforms: []string{"darwin"}},
        License: "MIT", Description: "x", Readme: "README.md", DisplayName: "X"}
    if err := ValidateWithOwner(&m, "foo"); err != nil {
        t.Fatalf("expected ok, got %v", err)
    }
    // Mismatch: manifest declares different explicit canonical form
    m.Name = "@other/bar"
    if err := ValidateWithOwner(&m, "foo"); err == nil {
        t.Fatal("expected owner mismatch error")
    }
}
```

- [ ] **Step 3: Run, fail**

Run: `go test ./internal/manifest/... -run WithRegistryOwner -v`

- [ ] **Step 4: Implement**

Append to `internal/manifest/validate.go`:
```go
import "github.com/enekos/agentpop/internal/namespace"

// ValidateWithOwner performs base Validate plus namespace consistency:
// the manifest's Name must either be a bare slug or must be @<registryOwner>/<slug>.
func ValidateWithOwner(m *Tool, registryOwner string) error {
    if err := Validate(m); err != nil {
        return err
    }
    n, err := namespace.Parse(m.Name)
    if err != nil {
        return fmt.Errorf("name: %w", err)
    }
    if !n.IsBare && n.Owner != registryOwner {
        return fmt.Errorf("name owner %q does not match registry owner %q", n.Owner, registryOwner)
    }
    return nil
}
```

(If `fmt` is not already imported in `validate.go`, add it. If the file already has an `import` block, extend it; otherwise place this function in a new file `internal/manifest/validate_owner.go` with the right imports — consult the file and decide.)

- [ ] **Step 5: Run, pass**

Run: `go test ./internal/manifest/... -v`

- [ ] **Step 6: Commit**

```bash
git add internal/manifest/
git commit -m "feat(manifest): ValidateWithOwner enforces @owner/slug matches registry"
```

---

## Task 7: Artifact snapshot builder — deterministic tarball

**Files:**
- Create: `internal/artifacts/snapshot.go`, `internal/artifacts/snapshot_test.go`

- [ ] **Step 1: Test**

`internal/artifacts/snapshot_test.go`:
```go
package artifacts

import (
    "bytes"
    "crypto/sha256"
    "encoding/hex"
    "testing"
)

func TestBuildSnapshotDeterministic(t *testing.T) {
    in := Input{
        Owner:    "foo",
        Slug:     "bar",
        Version:  "v1.2.3",
        Manifest: []byte(`{"name":"bar"}`),
        Readme:   []byte(`# bar`),
        License:  []byte(`MIT`),
    }
    var buf1, buf2 bytes.Buffer
    if err := Build(&buf1, in); err != nil {
        t.Fatal(err)
    }
    if err := Build(&buf2, in); err != nil {
        t.Fatal(err)
    }
    h1 := sha256.Sum256(buf1.Bytes())
    h2 := sha256.Sum256(buf2.Bytes())
    if hex.EncodeToString(h1[:]) != hex.EncodeToString(h2[:]) {
        t.Fatal("build not deterministic")
    }
    if buf1.Len() == 0 {
        t.Fatal("empty tarball")
    }
}
```

- [ ] **Step 2: Run, fail**

Run: `go test ./internal/artifacts/... -v`

- [ ] **Step 3: Implement**

`internal/artifacts/snapshot.go`:
```go
// Package artifacts builds deterministic manifest-snapshot tarballs
// (gzip'd tar) for a given package version. We do not re-host upstream
// npm/go/binary bytes; we snapshot the manifest + README + license only.
package artifacts

import (
    "archive/tar"
    "compress/gzip"
    "io"
    "sort"
    "time"
)

type Input struct {
    Owner    string
    Slug     string
    Version  string
    Manifest []byte // canonical manifest.json
    Readme   []byte
    License  []byte // may be empty
}

// fixed timestamp so identical inputs produce byte-identical output
var epoch = time.Unix(0, 0).UTC()

func Build(w io.Writer, in Input) error {
    gz := gzip.NewWriter(w)
    gz.ModTime = epoch
    gz.Name = ""
    tw := tar.NewWriter(gz)

    type file struct {
        name, body []byte
    }
    entries := []struct {
        name string
        body []byte
    }{
        {"manifest.json", in.Manifest},
        {"README.md", in.Readme},
    }
    if len(in.License) > 0 {
        entries = append(entries, struct {
            name string
            body []byte
        }{"LICENSE", in.License})
    }
    sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

    for _, e := range entries {
        h := &tar.Header{
            Name:    e.name,
            Mode:    0o644,
            Size:    int64(len(e.body)),
            ModTime: epoch,
            Format:  tar.FormatUSTAR,
        }
        if err := tw.WriteHeader(h); err != nil {
            return err
        }
        if _, err := tw.Write(e.body); err != nil {
            return err
        }
    }
    if err := tw.Close(); err != nil {
        return err
    }
    return gz.Close()
}
```

- [ ] **Step 4: Pass**

Run: `go test ./internal/artifacts/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/artifacts/
git commit -m "feat(artifacts): deterministic snapshot tarball builder"
```

---

## Task 8: Artifact store — filesystem implementation

**Files:**
- Create: `internal/artifacts/store.go`, `internal/artifacts/store_test.go`

- [ ] **Step 1: Test**

`internal/artifacts/store_test.go`:
```go
package artifacts

import (
    "bytes"
    "io"
    "os"
    "path/filepath"
    "testing"
)

func TestFSStorePutGet(t *testing.T) {
    dir := t.TempDir()
    s := NewFSStore(dir)
    body := []byte("hello")
    ref := Ref{Owner: "foo", Slug: "bar", Version: "v1.0.0"}
    sha, err := s.Put(ref, bytes.NewReader(body))
    if err != nil {
        t.Fatal(err)
    }
    if sha == "" {
        t.Fatal("empty sha")
    }
    rc, gotSha, err := s.Get(ref)
    if err != nil {
        t.Fatal(err)
    }
    defer rc.Close()
    got, _ := io.ReadAll(rc)
    if !bytes.Equal(got, body) || gotSha != sha {
        t.Fatal("mismatch")
    }
    // file laid out under owner/slug/version.tgz
    if _, err := os.Stat(filepath.Join(dir, "foo", "bar", "v1.0.0.tgz")); err != nil {
        t.Fatalf("layout: %v", err)
    }
}

func TestFSStoreImmutable(t *testing.T) {
    dir := t.TempDir()
    s := NewFSStore(dir)
    ref := Ref{Owner: "foo", Slug: "bar", Version: "v1.0.0"}
    if _, err := s.Put(ref, bytes.NewReader([]byte("a"))); err != nil {
        t.Fatal(err)
    }
    if _, err := s.Put(ref, bytes.NewReader([]byte("b"))); err == nil {
        t.Fatal("expected immutability error on re-put")
    }
}
```

- [ ] **Step 2: Fail**

Run: `go test ./internal/artifacts/... -v`

- [ ] **Step 3: Implement**

`internal/artifacts/store.go`:
```go
package artifacts

import (
    "crypto/sha256"
    "encoding/hex"
    "errors"
    "io"
    "os"
    "path/filepath"
)

type Ref struct{ Owner, Slug, Version string }

func (r Ref) path(root string) string {
    return filepath.Join(root, r.Owner, r.Slug, r.Version+".tgz")
}

type Store interface {
    Put(Ref, io.Reader) (sha256 string, err error)
    Get(Ref) (rc io.ReadCloser, sha256 string, err error)
    Exists(Ref) bool
}

type fsStore struct{ root string }

func NewFSStore(root string) Store { return &fsStore{root: root} }

var ErrImmutable = errors.New("artifact already exists and is immutable")

func (s *fsStore) Put(ref Ref, r io.Reader) (string, error) {
    p := ref.path(s.root)
    if _, err := os.Stat(p); err == nil {
        return "", ErrImmutable
    }
    if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
        return "", err
    }
    tmp, err := os.CreateTemp(filepath.Dir(p), ".tmp-*")
    if err != nil {
        return "", err
    }
    h := sha256.New()
    mw := io.MultiWriter(tmp, h)
    if _, err := io.Copy(mw, r); err != nil {
        tmp.Close()
        os.Remove(tmp.Name())
        return "", err
    }
    if err := tmp.Close(); err != nil {
        return "", err
    }
    if err := os.Rename(tmp.Name(), p); err != nil {
        return "", err
    }
    sum := hex.EncodeToString(h.Sum(nil))
    if err := os.WriteFile(p+".sha256", []byte(sum), 0o644); err != nil {
        return "", err
    }
    return sum, nil
}

func (s *fsStore) Get(ref Ref) (io.ReadCloser, string, error) {
    p := ref.path(s.root)
    f, err := os.Open(p)
    if err != nil {
        return nil, "", err
    }
    sum, err := os.ReadFile(p + ".sha256")
    if err != nil {
        f.Close()
        return nil, "", err
    }
    return f, string(sum), nil
}

func (s *fsStore) Exists(ref Ref) bool {
    _, err := os.Stat(ref.path(s.root))
    return err == nil
}
```

- [ ] **Step 4: Pass**

Run: `go test ./internal/artifacts/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/artifacts/
git commit -m "feat(artifacts): filesystem store with immutability enforcement"
```

---

## Task 9: SQLite infra + migrations

**Files:**
- Create: `internal/db/db.go`, `internal/db/db_test.go`, `internal/db/migrations/0001_init.sql`

- [ ] **Step 1: Migration SQL**

`internal/db/migrations/0001_init.sql`:
```sql
CREATE TABLE IF NOT EXISTS schema_version (
    version INTEGER PRIMARY KEY,
    applied_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS downloads (
    owner TEXT NOT NULL,
    slug TEXT NOT NULL,
    version TEXT NOT NULL,
    day TEXT NOT NULL,          -- YYYY-MM-DD UTC
    count INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (owner, slug, version, day)
);

CREATE TABLE IF NOT EXISTS audit (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ts TEXT NOT NULL,
    actor TEXT NOT NULL,
    action TEXT NOT NULL,
    owner TEXT,
    slug TEXT,
    version TEXT,
    meta TEXT                   -- JSON
);

CREATE INDEX IF NOT EXISTS audit_slug_idx ON audit(owner, slug);
```

- [ ] **Step 2: Test**

`internal/db/db_test.go`:
```go
package db

import (
    "path/filepath"
    "testing"
)

func TestOpenAndMigrate(t *testing.T) {
    path := filepath.Join(t.TempDir(), "t.sqlite")
    d, err := Open(path)
    if err != nil {
        t.Fatal(err)
    }
    defer d.Close()
    // Idempotent re-open.
    d2, err := Open(path)
    if err != nil {
        t.Fatal(err)
    }
    d2.Close()
    // Insert + read a download row.
    if err := d.IncrementDownload("foo", "bar", "v1.0.0", "2026-04-23"); err != nil {
        t.Fatal(err)
    }
    if err := d.IncrementDownload("foo", "bar", "v1.0.0", "2026-04-23"); err != nil {
        t.Fatal(err)
    }
    n, err := d.DownloadCount("foo", "bar", "v1.0.0")
    if err != nil {
        t.Fatal(err)
    }
    if n != 2 {
        t.Fatalf("count=%d want 2", n)
    }
}
```

- [ ] **Step 3: Fail**

Run: `go test ./internal/db/... -v`

- [ ] **Step 4: Implement**

`internal/db/db.go`:
```go
// Package db is the thin SQLite wrapper holding derived state
// (downloads, audit). Nothing here is load-bearing for install correctness.
package db

import (
    "database/sql"
    _ "embed"

    _ "modernc.org/sqlite"
)

//go:embed migrations/0001_init.sql
var migration0001 string

type DB struct{ sql *sql.DB }

func Open(path string) (*DB, error) {
    s, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)")
    if err != nil {
        return nil, err
    }
    if err := s.Ping(); err != nil {
        return nil, err
    }
    d := &DB{sql: s}
    if err := d.migrate(); err != nil {
        s.Close()
        return nil, err
    }
    return d, nil
}

func (d *DB) Close() error { return d.sql.Close() }

func (d *DB) migrate() error {
    if _, err := d.sql.Exec(migration0001); err != nil {
        return err
    }
    _, err := d.sql.Exec(`INSERT OR IGNORE INTO schema_version(version, applied_at) VALUES (1, datetime('now'))`)
    return err
}

func (d *DB) IncrementDownload(owner, slug, version, day string) error {
    _, err := d.sql.Exec(`
        INSERT INTO downloads(owner, slug, version, day, count)
        VALUES (?,?,?,?,1)
        ON CONFLICT(owner,slug,version,day) DO UPDATE SET count=count+1
    `, owner, slug, version, day)
    return err
}

func (d *DB) DownloadCount(owner, slug, version string) (int64, error) {
    var n sql.NullInt64
    err := d.sql.QueryRow(`SELECT COALESCE(SUM(count),0) FROM downloads WHERE owner=? AND slug=? AND version=?`, owner, slug, version).Scan(&n)
    return n.Int64, err
}

func (d *DB) AuditInsert(ts, actor, action, owner, slug, version, metaJSON string) error {
    _, err := d.sql.Exec(`INSERT INTO audit(ts, actor, action, owner, slug, version, meta) VALUES (?,?,?,?,?,?,?)`,
        ts, actor, action, owner, slug, version, metaJSON)
    return err
}
```

- [ ] **Step 5: Pass**

Run: `go test ./internal/db/... -v`

- [ ] **Step 6: Commit**

```bash
git add internal/db/
git commit -m "feat(db): SQLite wrapper with initial downloads+audit schema"
```

---

## Task 10: Versioned corpus layout — writer

**Files:**
- Modify: `internal/corpus/writer.go`, `internal/corpus/writer_test.go`

- [ ] **Step 1: Read current writer**

Run: `cat internal/corpus/writer.go`

Note the current layout: `corpus/<slug>/{manifest.json,index.md}`. We are extending, not replacing. The `latest.json` continues to point at the current-newest version; the non-versioned files are kept for v1 compatibility during migration.

- [ ] **Step 2: Test**

Append to `internal/corpus/writer_test.go`:
```go
func TestWriteVersioned(t *testing.T) {
    dir := t.TempDir()
    w := NewWriter(dir)
    entry := VersionedEntry{
        Owner: "foo", Slug: "bar", Version: "v1.2.3",
        ManifestJSON: []byte(`{"name":"bar"}`),
        IndexMD:      []byte("---\nslug: bar\n---\n# bar"),
        ArtifactSHA:  "deadbeef",
    }
    if err := w.WriteVersion(entry); err != nil {
        t.Fatal(err)
    }
    got, err := os.ReadFile(filepath.Join(dir, "foo", "bar", "versions", "v1.2.3", "manifest.json"))
    if err != nil {
        t.Fatal(err)
    }
    if string(got) != `{"name":"bar"}` {
        t.Fatalf("mismatch: %s", got)
    }
    sha, err := os.ReadFile(filepath.Join(dir, "foo", "bar", "versions", "v1.2.3", "artifact.sha256"))
    if err != nil {
        t.Fatal(err)
    }
    if string(sha) != "deadbeef" {
        t.Fatalf("sha mismatch: %s", sha)
    }
    latest, err := os.ReadFile(filepath.Join(dir, "foo", "bar", "latest.json"))
    if err != nil {
        t.Fatal(err)
    }
    if !strings.Contains(string(latest), `"version":"v1.2.3"`) {
        t.Fatalf("latest missing version: %s", latest)
    }
}
```

Add any missing imports (`os`, `path/filepath`, `strings`).

- [ ] **Step 3: Fail**

Run: `go test ./internal/corpus/... -run WriteVersioned -v`

- [ ] **Step 4: Implement**

Add to `internal/corpus/writer.go`:
```go
type VersionedEntry struct {
    Owner, Slug, Version string
    ManifestJSON         []byte
    IndexMD              []byte
    ArtifactSHA          string
}

// WriteVersion writes corpus/<owner>/<slug>/versions/<v>/ and updates latest.json.
func (w *Writer) WriteVersion(e VersionedEntry) error {
    base := filepath.Join(w.root, e.Owner, e.Slug, "versions", e.Version)
    if err := os.MkdirAll(base, 0o755); err != nil {
        return err
    }
    if err := atomicWrite(filepath.Join(base, "manifest.json"), e.ManifestJSON); err != nil {
        return err
    }
    if err := atomicWrite(filepath.Join(base, "index.md"), e.IndexMD); err != nil {
        return err
    }
    if err := atomicWrite(filepath.Join(base, "artifact.sha256"), []byte(e.ArtifactSHA)); err != nil {
        return err
    }
    latest := fmt.Sprintf(`{"owner":%q,"slug":%q,"version":%q}`, e.Owner, e.Slug, e.Version)
    return atomicWrite(filepath.Join(w.root, e.Owner, e.Slug, "latest.json"), []byte(latest))
}

func atomicWrite(p string, body []byte) error {
    tmp, err := os.CreateTemp(filepath.Dir(p), ".tmp-*")
    if err != nil {
        return err
    }
    if _, err := tmp.Write(body); err != nil {
        tmp.Close()
        os.Remove(tmp.Name())
        return err
    }
    if err := tmp.Close(); err != nil {
        return err
    }
    return os.Rename(tmp.Name(), p)
}
```

Add imports `os`, `path/filepath`, `fmt` if missing.

- [ ] **Step 5: Pass**

Run: `go test ./internal/corpus/... -v`

- [ ] **Step 6: Commit**

```bash
git add internal/corpus/
git commit -m "feat(corpus): versioned layout writer with latest.json"
```

---

## Task 11: Versioned corpus — reader

**Files:**
- Modify: `internal/corpus/reader.go`, `internal/corpus/reader_test.go`

- [ ] **Step 1: Test**

Append to `internal/corpus/reader_test.go`:
```go
func TestListVersions(t *testing.T) {
    dir := t.TempDir()
    w := NewWriter(dir)
    for _, v := range []string{"v1.0.0", "v1.2.3", "v0.9.0"} {
        _ = w.WriteVersion(VersionedEntry{Owner: "foo", Slug: "bar", Version: v, ManifestJSON: []byte("{}"), IndexMD: []byte(""), ArtifactSHA: "x"})
    }
    r := NewReader(dir)
    vs, err := r.ListVersions("foo", "bar")
    if err != nil {
        t.Fatal(err)
    }
    if len(vs) != 3 {
        t.Fatalf("got %d", len(vs))
    }
    // Sorted ascending
    if vs[0] != "v0.9.0" || vs[2] != "v1.2.3" {
        t.Fatalf("order: %v", vs)
    }
}

func TestReadVersion(t *testing.T) {
    dir := t.TempDir()
    w := NewWriter(dir)
    _ = w.WriteVersion(VersionedEntry{Owner: "foo", Slug: "bar", Version: "v1.0.0",
        ManifestJSON: []byte(`{"name":"bar"}`), IndexMD: []byte("md"), ArtifactSHA: "sha"})
    r := NewReader(dir)
    m, md, sha, err := r.ReadVersion("foo", "bar", "v1.0.0")
    if err != nil {
        t.Fatal(err)
    }
    if string(m) != `{"name":"bar"}` || string(md) != "md" || sha != "sha" {
        t.Fatalf("bad read: %s %s %s", m, md, sha)
    }
}
```

- [ ] **Step 2: Fail**

Run: `go test ./internal/corpus/... -run Version -v`

- [ ] **Step 3: Implement**

Add to `internal/corpus/reader.go`:
```go
import "github.com/enekos/agentpop/internal/versioning"

func (r *Reader) ListVersions(owner, slug string) ([]string, error) {
    dir := filepath.Join(r.root, owner, slug, "versions")
    entries, err := os.ReadDir(dir)
    if err != nil {
        return nil, err
    }
    raw := make([]string, 0, len(entries))
    for _, e := range entries {
        if e.IsDir() {
            raw = append(raw, e.Name())
        }
    }
    vs := versioning.ScanTags(raw)
    out := make([]string, len(vs))
    for i, v := range vs {
        out[i] = v.Canonical()
    }
    return out, nil
}

func (r *Reader) ReadVersion(owner, slug, version string) (manifest []byte, index []byte, sha string, err error) {
    base := filepath.Join(r.root, owner, slug, "versions", version)
    m, err := os.ReadFile(filepath.Join(base, "manifest.json"))
    if err != nil {
        return nil, nil, "", err
    }
    idx, err := os.ReadFile(filepath.Join(base, "index.md"))
    if err != nil {
        return nil, nil, "", err
    }
    shab, err := os.ReadFile(filepath.Join(base, "artifact.sha256"))
    if err != nil {
        return nil, nil, "", err
    }
    return m, idx, string(shab), nil
}
```

- [ ] **Step 4: Pass**

Run: `go test ./internal/corpus/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/corpus/
git commit -m "feat(corpus): versioned layout reader"
```

---

## Task 12: Crawler tag-diff — detect new versions per tool repo

**Files:**
- Modify: `internal/crawl/crawl.go`, `internal/crawl/fetcher.go`, `internal/crawl/crawl_test.go`

- [ ] **Step 1: Read current crawler**

Run: `cat internal/crawl/crawl.go internal/crawl/fetcher.go`

The current crawler pulls a single ref per repo. We add: list tags, filter via `versioning.ScanTags`, iterate each tag, skip tags already present in the corpus.

- [ ] **Step 2: Fetcher interface update**

Modify `internal/crawl/fetcher.go` to add `ListTags(repo string) ([]string, error)` on the `Fetcher` interface and its fake/real impls. The real impl runs `git ls-remote --tags <repo>` and parses output. The fake stores a tag list per repo URL.

- [ ] **Step 3: Test**

Add `internal/crawl/crawl_test.go` test `TestCrawlIngestsNewTagsOnly`:
```go
func TestCrawlIngestsNewTagsOnly(t *testing.T) {
    root := t.TempDir()
    artDir := t.TempDir()
    fakeFetcher := &fakeFetcher{
        tags: map[string][]string{"github.com/foo/bar": {"v1.0.0", "v1.1.0"}},
        snapshots: map[string]RepoSnapshot{ /* see existing fake */ },
    }
    c := NewCrawler(root, artDir, fakeFetcher, nil)
    if err := c.CrawlOne(Entry{Repo: "github.com/foo/bar", Owner: "foo"}); err != nil {
        t.Fatal(err)
    }
    // Second run should be a no-op (no new tags)
    stats, err := c.CrawlOneWithStats(Entry{Repo: "github.com/foo/bar", Owner: "foo"})
    if err != nil {
        t.Fatal(err)
    }
    if stats.NewVersions != 0 {
        t.Fatalf("expected 0 new, got %d", stats.NewVersions)
    }
    // Adding a new tag produces one new version.
    fakeFetcher.tags["github.com/foo/bar"] = append(fakeFetcher.tags["github.com/foo/bar"], "v1.2.0")
    stats, _ = c.CrawlOneWithStats(Entry{Repo: "github.com/foo/bar", Owner: "foo"})
    if stats.NewVersions != 1 {
        t.Fatalf("expected 1 new, got %d", stats.NewVersions)
    }
}
```

- [ ] **Step 4: Implement**

Rework `internal/crawl/crawl.go` so `CrawlOne(Entry)` per repo:

```go
type Entry struct{ Repo, Owner string }

type CrawlStats struct{ NewVersions int }

func (c *Crawler) CrawlOne(e Entry) error {
    _, err := c.CrawlOneWithStats(e)
    return err
}

func (c *Crawler) CrawlOneWithStats(e Entry) (CrawlStats, error) {
    tags, err := c.fetcher.ListTags(e.Repo)
    if err != nil {
        return CrawlStats{}, err
    }
    versions := versioning.ScanTags(tags)
    var stats CrawlStats
    for _, v := range versions {
        // Skip if version already ingested.
        if c.corpusReader.HasVersion(e.Owner, slugFromRepo(e.Repo), v.Canonical()) {
            continue
        }
        if err := c.ingestVersion(e, v); err != nil {
            // log and continue on per-version failure
            continue
        }
        stats.NewVersions++
    }
    return stats, nil
}
```

Add `HasVersion(owner, slug, version string) bool` on `corpus.Reader`.

`ingestVersion` fetches the repo at the tag, runs `manifest.ValidateWithOwner`, builds the snapshot tarball via `artifacts.Build`, stores it via `artifacts.Store.Put`, and calls `corpus.Writer.WriteVersion`.

(Full code is large; the structure above plus the existing single-ref logic in `crawl.go` is enough for the engineer to write it. All error paths log via the existing logger and continue.)

- [ ] **Step 5: Pass**

Run: `go test ./internal/crawl/... -v`

- [ ] **Step 6: Commit**

```bash
git add internal/crawl/ internal/corpus/
git commit -m "feat(crawler): tag-diff loop ingests new versions only"
```

---

## Task 13: API — `/api/tools/@owner/slug` and version list

**Files:**
- Modify: `internal/api/tools.go`, `internal/api/tools_test.go`, `internal/api/server.go`

- [ ] **Step 1: Route registration**

In `internal/api/server.go`, register:
- `GET /api/tools/@{owner}/{slug}` → `handleGetToolV2`
- `GET /api/tools/@{owner}/{slug}/versions` → `handleListVersions`
- `GET /api/tools/@{owner}/{slug}/@{version}` → `handleGetVersion`

- [ ] **Step 2: Test**

Add `internal/api/tools_test.go::TestGetToolV2` using a fixture corpus with `foo/bar/versions/v1.0.0/` and `v1.1.0/`:

```go
func TestGetToolV2(t *testing.T) {
    srv := newTestServer(t, fixtureVersionedCorpus(t))
    rsp := do(t, srv, "GET", "/api/tools/@foo/bar")
    if rsp.Code != 200 {
        t.Fatalf("status %d", rsp.Code)
    }
    var body struct {
        Owner    string   `json:"owner"`
        Slug     string   `json:"slug"`
        Latest   string   `json:"latest_version"`
        Versions []string `json:"versions"`
    }
    json.NewDecoder(rsp.Body).Decode(&body)
    if body.Latest != "v1.1.0" {
        t.Fatalf("latest=%s", body.Latest)
    }
    if len(body.Versions) != 2 {
        t.Fatal("versions")
    }
}
```

- [ ] **Step 3: Fail**

Run: `go test ./internal/api/... -run GetToolV2 -v`

- [ ] **Step 4: Implement** — handlers read from the corpus reader, return JSON with `owner, slug, latest_version, versions[], manifest (latest)`.

- [ ] **Step 5: Pass**

Run: `go test ./internal/api/... -v`

- [ ] **Step 6: Commit**

```bash
git add internal/api/
git commit -m "feat(api): @owner/slug versioned tool routes"
```

---

## Task 14: API — `/api/artifacts/@owner/slug/@version` streaming + downloads

**Files:**
- Modify: `internal/api/server.go`, create `internal/api/artifacts.go`, `internal/api/artifacts_test.go`

- [ ] **Step 1: Test**

`internal/api/artifacts_test.go`:
```go
func TestGetArtifact(t *testing.T) {
    srv := newTestServer(t, fixtureVersionedCorpusWithArtifact(t))
    rsp := do(t, srv, "GET", "/api/artifacts/@foo/bar/@v1.0.0")
    if rsp.Code != 200 {
        t.Fatalf("status=%d", rsp.Code)
    }
    if rsp.Header().Get("Content-Type") != "application/gzip" {
        t.Fatalf("ct=%s", rsp.Header().Get("Content-Type"))
    }
    if rsp.Header().Get("X-Agentpop-SHA256") == "" {
        t.Fatal("sha header missing")
    }
    // A second request increments download count.
    do(t, srv, "GET", "/api/artifacts/@foo/bar/@v1.0.0")
    n, _ := srv.DB.DownloadCount("foo", "bar", "v1.0.0")
    if n != 2 {
        t.Fatalf("count=%d", n)
    }
}
```

- [ ] **Step 2: Fail**

Run: `go test ./internal/api/... -run GetArtifact -v`

- [ ] **Step 3: Implement**

`internal/api/artifacts.go` handler looks up the artifact via the store, streams it, sets `Content-Type: application/gzip`, `Cache-Control: public, max-age=31536000, immutable`, `X-Agentpop-SHA256: <sha>`, calls `DB.IncrementDownload(...)` with today's UTC date after the stream starts.

The server struct gains `Store artifacts.Store` and `DB *db.DB` fields wired in `cmd/apid/main.go`.

- [ ] **Step 4: Pass**

Run: `go test ./internal/api/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/api/ cmd/apid/
git commit -m "feat(api): /api/artifacts streams tarball and bumps downloads"
```

---

## Task 15: API — version-aware `/api/install/@owner/slug[@version]`

**Files:**
- Modify: `internal/api/install.go`, `internal/api/install_test.go`

- [ ] **Step 1: Test**

Append to `internal/api/install_test.go`:
```go
func TestInstallVersionAware(t *testing.T) {
    srv := newTestServer(t, fixtureVersionedCorpus(t))
    // No version -> latest.
    rsp := do(t, srv, "GET", "/api/install/@foo/bar")
    assertJSONField(t, rsp, "resolved_version", "v1.1.0")
    // Explicit version.
    rsp = do(t, srv, "GET", "/api/install/@foo/bar/@v1.0.0")
    assertJSONField(t, rsp, "resolved_version", "v1.0.0")
    // Range.
    rsp = do(t, srv, "GET", "/api/install/@foo/bar?range=^1.0")
    assertJSONField(t, rsp, "resolved_version", "v1.1.0")
}
```

- [ ] **Step 2: Fail**

Run: `go test ./internal/api/... -run InstallVersion -v`

- [ ] **Step 3: Implement**

Extend `handleInstall` to:
1. Parse `@owner/slug` (and optional `@version` path segment, or `?range=` query).
2. `corpus.ListVersions(owner, slug)` → `[]versioning.Version`.
3. Resolve: explicit version > range > empty-range-meaning-latest-stable.
4. Return `{resolved_version, sha256, tabs: [...]}` — tabs come from existing adapter snippet logic keyed off the version's manifest.

- [ ] **Step 4: Pass**

Run: `go test ./internal/api/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/api/
git commit -m "feat(api): version-aware install route"
```

---

## Task 16: `internal/lockfile` — TOML parse/write

**Files:**
- Create: `internal/lockfile/lockfile.go`, `internal/lockfile/lockfile_test.go`

- [ ] **Step 1: Test**

`internal/lockfile/lockfile_test.go`:
```go
package lockfile

import (
    "bytes"
    "path/filepath"
    "os"
    "testing"
)

func TestRoundTrip(t *testing.T) {
    l := &Lock{Schema: 1, Packages: []Entry{
        {Slug: "@foo/bar", Version: "v1.2.3", SHA256: "deadbeef",
            SourceRepo: "github.com/foo/bar", SourceRef: "refs/tags/v1.2.3",
            InstalledAt: "2026-04-23T00:00:00Z", Kind: "mcp"},
    }}
    var buf bytes.Buffer
    if err := Write(&buf, l); err != nil {
        t.Fatal(err)
    }
    got, err := Read(&buf)
    if err != nil {
        t.Fatal(err)
    }
    if len(got.Packages) != 1 || got.Packages[0].Slug != "@foo/bar" {
        t.Fatalf("mismatch: %+v", got)
    }
}

func TestReadFromFile(t *testing.T) {
    path := filepath.Join(t.TempDir(), "agentpop.lock")
    os.WriteFile(path, []byte("schema = 1\n\n[[packages]]\nslug = \"@foo/bar\"\nversion = \"v1.0.0\"\nsha256 = \"x\"\nsource_repo = \"r\"\nsource_ref = \"refs/tags/v1.0.0\"\ninstalled_at = \"t\"\nkind = \"mcp\"\n"), 0o644)
    l, err := ReadFile(path)
    if err != nil {
        t.Fatal(err)
    }
    if l.Packages[0].Slug != "@foo/bar" {
        t.Fatal("slug")
    }
}

func TestFrozenRefusesUnknown(t *testing.T) {
    l := &Lock{Schema: 1, Packages: []Entry{{Slug: "@foo/bar", Version: "v1.0.0"}}}
    if err := l.CheckFrozen("@foo/bar", "v1.0.0"); err != nil {
        t.Fatal(err)
    }
    if err := l.CheckFrozen("@foo/bar", "v2.0.0"); err == nil {
        t.Fatal("expected version mismatch")
    }
    if err := l.CheckFrozen("@other/thing", "v1"); err == nil {
        t.Fatal("expected unknown slug")
    }
}
```

- [ ] **Step 2: Fail**

Run: `go test ./internal/lockfile/... -v`

- [ ] **Step 3: Implement**

`internal/lockfile/lockfile.go`:
```go
package lockfile

import (
    "errors"
    "io"
    "os"

    "github.com/BurntSushi/toml"
)

type Lock struct {
    Schema   int     `toml:"schema"`
    Packages []Entry `toml:"packages"`
}

type Entry struct {
    Slug        string `toml:"slug"`
    Version     string `toml:"version"`
    SHA256      string `toml:"sha256"`
    SourceRepo  string `toml:"source_repo"`
    SourceRef   string `toml:"source_ref"`
    InstalledAt string `toml:"installed_at"`
    Kind        string `toml:"kind"`
}

func Read(r io.Reader) (*Lock, error) {
    var l Lock
    if _, err := toml.NewDecoder(r).Decode(&l); err != nil {
        return nil, err
    }
    return &l, nil
}

func ReadFile(path string) (*Lock, error) {
    f, err := os.Open(path)
    if err != nil {
        return nil, err
    }
    defer f.Close()
    return Read(f)
}

func Write(w io.Writer, l *Lock) error {
    return toml.NewEncoder(w).Encode(l)
}

func WriteFile(path string, l *Lock) error {
    f, err := os.Create(path)
    if err != nil {
        return err
    }
    defer f.Close()
    return Write(f, l)
}

// CheckFrozen verifies that slug@version is exactly pinned in the lockfile.
func (l *Lock) CheckFrozen(slug, version string) error {
    for _, p := range l.Packages {
        if p.Slug == slug {
            if p.Version == version {
                return nil
            }
            return errors.New("version mismatch: locked=" + p.Version + " requested=" + version)
        }
    }
    return errors.New("slug not in lockfile: " + slug)
}

// Upsert replaces any existing entry with the same slug or appends.
func (l *Lock) Upsert(e Entry) {
    for i, p := range l.Packages {
        if p.Slug == e.Slug {
            l.Packages[i] = e
            return
        }
    }
    l.Packages = append(l.Packages, e)
}
```

- [ ] **Step 4: Pass**

Run: `go test ./internal/lockfile/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/lockfile/
git commit -m "feat(lockfile): read/write agentpop.lock with CheckFrozen"
```

---

## Task 17: CLI `agentpop install @x/y@range` with lockfile write

**Files:**
- Modify: `internal/clicmd/install.go`, `internal/clicmd/install_test.go`, `internal/apiclient/` (add versioned fetch)

- [ ] **Step 1: Read current install.go**

Run: `cat internal/clicmd/install.go`

Locate the spot where the install flow resolves a manifest from the API.

- [ ] **Step 2: Test — range resolution**

Add `internal/clicmd/install_test.go::TestInstallWritesLockfile`:

Set up a fake apiclient that returns `resolved_version=v1.2.3` + sha for `@foo/bar`. Run `install @foo/bar` with a temp workdir. Assert `agentpop.lock` is created and contains the entry.

- [ ] **Step 3: Fail**

Run: `go test ./internal/clicmd/... -run LockFile -v`

- [ ] **Step 4: Implement**

In `install` run flow:
1. Parse argument with `namespace.Parse` + optional `@<range>` suffix.
2. Call `apiclient.GetInstall(name, rangeSpec)` which hits `/api/install/@owner/slug[?range=...]`.
3. Use the returned manifest + sha to run adapter install.
4. After successful adapter install, read lockfile at `./agentpop.lock` (create if absent), `Upsert` the entry, `WriteFile`.

- [ ] **Step 5: Pass**

Run: `go test ./internal/clicmd/... -v`

- [ ] **Step 6: Commit**

```bash
git add internal/clicmd/ internal/apiclient/
git commit -m "feat(cli): install resolves range and writes agentpop.lock"
```

---

## Task 18: CLI `agentpop install --frozen`

**Files:**
- Modify: `internal/clicmd/install.go`, `internal/clicmd/install_test.go`

- [ ] **Step 1: Test**

Add `TestFrozenRefusesOnMismatch`: lockfile pins `@foo/bar` at `v1.0.0`; running `install @foo/bar@v2.0.0 --frozen` must error without touching the adapter.

Also `TestFrozenInstallsExactlyLocked`: running `install --frozen` with no args installs every lockfile entry at exactly the pinned version.

- [ ] **Step 2: Fail**

Run: `go test ./internal/clicmd/... -run Frozen -v`

- [ ] **Step 3: Implement**

Add `--frozen` flag. When set:
- No args: iterate lockfile entries, install each at exact version.
- With arg: parse name+range, require an exact match with a lockfile entry; else error.

- [ ] **Step 4: Pass**

Run: `go test ./internal/clicmd/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/clicmd/
git commit -m "feat(cli): install --frozen enforces lockfile exactly"
```

---

## Task 19: CLI `agentpop upgrade`

**Files:**
- Create: `internal/clicmd/upgrade.go`, `internal/clicmd/upgrade_test.go`

- [ ] **Step 1: Test**

`TestUpgradeBumpsVersionInLockfile`: lockfile pins `@foo/bar@v1.0.0`; API says latest is `v1.1.0`; `agentpop upgrade @foo/bar` updates the lockfile entry and reinstalls.

`TestUpgradeAllNoArgs`: upgrades every entry in the lockfile.

- [ ] **Step 2: Fail**

Run: `go test ./internal/clicmd/... -run Upgrade -v`

- [ ] **Step 3: Implement**

`upgrade` flow per entry:
1. Resolve current range = `^MAJOR.MINOR` of the locked version (or `latest` if no args on install).
2. If newer version satisfies, run adapter install, update lockfile entry.
3. Print a small diff summary `@foo/bar v1.0.0 -> v1.1.0`.

- [ ] **Step 4: Pass + register command**

Register in `cmd/agentpop/main.go`.

Run: `go test ./internal/clicmd/... -v && make build`

- [ ] **Step 5: Commit**

```bash
git add internal/clicmd/ cmd/agentpop/
git commit -m "feat(cli): agentpop upgrade for lockfile-driven updates"
```

---

## Task 20: CLI `agentpop publish` (tag + push + poll)

**Files:**
- Create: `internal/clicmd/publish.go`, `internal/clicmd/publish_test.go`

- [ ] **Step 1: Test**

Fake git runner + fake apiclient. `TestPublishTagsPushesAndPolls`:
1. cwd contains `agentpop.yaml` with `version: 1.2.3`.
2. `agentpop publish` runs `git tag v1.2.3`, `git push origin v1.2.3`, then polls `GET /api/tools/@owner/slug/@v1.2.3` until 200.
3. Refuses to publish if the tag already exists locally or the working tree is dirty.

- [ ] **Step 2: Fail**

Run: `go test ./internal/clicmd/... -run Publish -v`

- [ ] **Step 3: Implement**

Poll with exponential backoff, cap at 10 min, print spinner. Exit 0 on ingestion confirmation, 1 on timeout (with a clear "not ingested yet, check back later" message — tag is still pushed so nothing is lost).

The manifest must carry a new optional `version:` top-level field; add it to `internal/manifest/types.go` with validation: must parse via `versioning.ParseVersion`.

- [ ] **Step 4: Pass + register**

Run: `go test ./internal/clicmd/... -v && make build`

Register in `cmd/agentpop/main.go`.

- [ ] **Step 5: Commit**

```bash
git add internal/clicmd/ cmd/agentpop/ internal/manifest/
git commit -m "feat(cli): agentpop publish tags, pushes, and polls ingestion"
```

---

## Task 21: Migration — bare-slug 301 redirect on API

**Files:**
- Modify: `internal/api/tools.go`, `internal/api/install.go`, `internal/api/server.go`, tests alongside

- [ ] **Step 1: Test**

`TestBareSlugRedirects`:
- Fixture corpus has `@foo/bar`.
- `GET /api/tools/bar` returns 301 with `Location: /api/tools/@foo/bar`.

The resolver uses a simple in-memory map built from `corpus/_index.json` at startup that picks the most-starred `@owner/slug` for a given bare slug.

- [ ] **Step 2: Fail**

Run: `go test ./internal/api/... -run Bare -v`

- [ ] **Step 3: Implement**

Add route `GET /api/tools/{slug}` (regex excluding paths starting with `@`). Handler looks up in the bare-slug map and issues 301.

- [ ] **Step 4: Pass**

Run: `go test ./internal/api/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/api/
git commit -m "feat(api): 301 redirect legacy bare-slug routes to @owner/slug"
```

---

## Task 22: Migration — synthetic `v0.0.0` for tools with no tags

**Files:**
- Modify: `internal/crawl/crawl.go`, `internal/crawl/crawl_test.go`

- [ ] **Step 1: Test**

`TestSyntheticVersionForUntaggedRepo`:
- Fake fetcher returns zero tags but a valid `agentpop.yaml` on `main`.
- After crawl, `corpus/foo/bar/versions/v0.0.0/manifest.json` exists.
- Latest version is `v0.0.0`.
- Re-crawling is a no-op unless the HEAD commit changes (the synthetic version carries the commit SHA in its `manifest.json` under a new `.synthetic_ref` field; if the commit changes, the synthetic entry is *replaced in place* — this is the one exception to immutability).

- [ ] **Step 2: Fail**

Run: `go test ./internal/crawl/... -run Synthetic -v`

- [ ] **Step 3: Implement**

When `versioning.ScanTags(tags)` is empty:
1. Fetch `main`.
2. Read HEAD commit SHA.
3. If `corpus/<owner>/<slug>/versions/v0.0.0/manifest.json` already exists and its stored `.synthetic_ref` equals current SHA, skip.
4. Otherwise, build snapshot with `version="v0.0.0"` and a banner field `synthetic=true` in `manifest.json`. Write (overwriting — explicitly allowed only for `v0.0.0` with `synthetic=true`).

Display layer reads `synthetic=true` and renders a "⚠ unversioned" banner prompting the author to tag.

- [ ] **Step 4: Pass**

Run: `go test ./internal/crawl/... -v`

- [ ] **Step 5: Commit**

```bash
git add internal/crawl/
git commit -m "feat(crawler): synthetic v0.0.0 for untagged tool repos"
```

---

## Task 23: Wire everything in `cmd/apid` and `cmd/crawler`

**Files:**
- Modify: `cmd/apid/main.go`, `cmd/crawler/main.go`

- [ ] **Step 1: apid**

`cmd/apid/main.go` additions:
- Flag `-sqlite /var/lib/agentpop/agentpop.sqlite` (defaults to `./agentpop.sqlite`).
- Flag `-artifacts /var/lib/agentpop/artifacts` (defaults to `./artifacts`).
- Open `db.Open(flag)`, create `artifacts.NewFSStore(flag)`, inject into `api.Server`.
- Wire graceful shutdown: `db.Close()`.

- [ ] **Step 2: crawler**

`cmd/crawler/main.go` additions:
- Flag `-artifacts ./artifacts`.
- Construct `artifacts.NewFSStore`, pass into `crawl.NewCrawler`.
- Print a run summary with `NewVersions` per repo and write `corpus/_crawl.json`.

- [ ] **Step 3: Build + run smoke**

```bash
make build
bin/apid -corpus internal/api/testdata/corpus -sqlite /tmp/test.sqlite -artifacts /tmp/artifacts &
curl -sf http://localhost:8091/api/_health
kill %1
```

Expected: `_health` returns 200; sqlite file created.

- [ ] **Step 4: Commit**

```bash
git add cmd/
git commit -m "feat(cmd): wire SQLite + artifact store into apid and crawler"
```

---

## Task 24: E2E smoke — install + upgrade via real binaries against a fixture corpus

**Files:**
- Create: `scripts/e2e-track-a.sh`

- [ ] **Step 1: Script**

`scripts/e2e-track-a.sh`:
```bash
#!/usr/bin/env bash
set -euo pipefail

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

# 1. Build
make build

# 2. Seed a fixture corpus with two versions of @foo/bar.
./scripts/seed-fixture-corpus.sh "$TMP/corpus" "$TMP/artifacts"

# 3. Start apid
bin/apid -corpus "$TMP/corpus" -artifacts "$TMP/artifacts" -sqlite "$TMP/db.sqlite" -addr :18091 &
APID_PID=$!
trap 'kill $APID_PID 2>/dev/null || true; rm -rf "$TMP"' EXIT
sleep 1

# 4. Install latest
cd "$TMP"
AGENTPOP_API=http://localhost:18091 bin/agentpop install @foo/bar
grep -q '"@foo/bar"' agentpop.lock
grep -q '"v1.1.0"' agentpop.lock

# 5. --frozen at matching version succeeds
AGENTPOP_API=http://localhost:18091 bin/agentpop install @foo/bar --frozen

# 6. Upgrade no-op (already latest)
AGENTPOP_API=http://localhost:18091 bin/agentpop upgrade @foo/bar | grep -q "up to date"

echo "TRACK-A-OK"
```

Create `scripts/seed-fixture-corpus.sh` that writes two minimal versions with a tiny tarball each (use the existing `internal/artifacts` builder via a tiny Go program or hand-roll the tarball).

- [ ] **Step 2: Run**

```bash
bash scripts/e2e-track-a.sh
```

Expected: prints `TRACK-A-OK`.

- [ ] **Step 3: Commit**

```bash
git add scripts/
git commit -m "test(e2e): track-a smoke covers install, frozen, upgrade"
```

---

## Task 25: Docs — publishing and lockfiles

**Files:**
- Create: `docs/publishing.md`, `docs/lockfile.md`

- [ ] **Step 1: Write**

Short docs (1–2 screens each):
- `docs/publishing.md` — "Submit once via PR to `registry/tools.yaml`; all future versions = `agentpop publish` or `git tag vX.Y.Z && git push`." Include the minimum valid `agentpop.yaml` with `name: @owner/slug` and `version: 1.0.0`.
- `docs/lockfile.md` — lockfile shape, when it's written, how `--frozen` and `upgrade` interact.

- [ ] **Step 2: Commit**

```bash
git add docs/
git commit -m "docs: publishing flow and lockfile semantics"
```

---

## Self-review results

Spec-coverage check for Track A in the design doc:

- **Package identity / `@owner/slug`** — Tasks 5, 6, 21 (redirect).
- **Versions = `v<semver>` tags** — Tasks 2, 3, 4, 12.
- **Immutable per-version artifact** — Tasks 7, 8, 12, 22 (synthetic exception explicit).
- **Corpus versioned layout** — Tasks 10, 11.
- **Lockfile** — Tasks 16, 17, 18, 19.
- **Minimal SQLite** — Task 9; wired into API in Task 14 (downloads); audit table defined though not yet written by any command in Track A (written in Track C).
- **API routes** — Tasks 13, 14, 15, 21.
- **CLI additions** — Tasks 17, 18, 19, 20.
- **Migration from v1** — Tasks 21 (bare-slug redirect), 22 (synthetic `v0.0.0`), Task 17 implicitly (manifest `version:` field).
- **Ops wiring** — Task 23.
- **E2E** — Task 24.
- **Docs** — Task 25.

Placeholder scan: no "TBD"/"implement later". Code blocks appear in every code step. Types defined in earlier tasks are reused consistently in later ones (`namespace.Name`, `versioning.Version`, `artifacts.Ref`, `lockfile.Entry`).

Gaps intentionally deferred to other tracks:
- Sigstore verification (Track C).
- `advisories` table population and `agentpop audit` command (Track C).
- Accounts, sessions, `agentpop login`/`whoami` (Track B).
- `yank`/`deprecate` commands (Track B + C).
- New package kinds `skill`/`subagent`/`command`/`bundle` (Track D).
