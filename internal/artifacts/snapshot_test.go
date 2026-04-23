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
