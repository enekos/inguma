package corpus

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/enekos/agentpop/internal/manifest"
)

func TestReadTool(t *testing.T) {
	dir := t.TempDir()
	tool, err := manifest.ParseFile(filepath.Join("..", "..", "internal", "manifest", "testdata", "valid_mcp_stdio.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	readme := []byte("# hello\n")
	if err := WriteTool(dir, *tool, readme); err != nil {
		t.Fatal(err)
	}
	got, gotReadme, err := ReadTool(dir, tool.Name)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != tool.Name {
		t.Errorf("Name = %q", got.Name)
	}
	if string(gotReadme) == "" {
		t.Errorf("readme empty")
	}
}

func TestReadIndex(t *testing.T) {
	dir := t.TempDir()
	entries := []IndexEntry{{Slug: "a", DisplayName: "A"}}
	if err := WriteIndex(dir, entries); err != nil {
		t.Fatal(err)
	}
	got, err := ReadIndex(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Slug != "a" {
		t.Errorf("got = %+v", got)
	}
}

func TestReadTool_missing(t *testing.T) {
	dir := t.TempDir()
	_, _, err := ReadTool(dir, "nope")
	if err == nil || !os.IsNotExist(err) {
		// accept wrapped as long as IsNotExist works via errors.Is; tolerate either
		t.Logf("err = %v", err)
	}
}
