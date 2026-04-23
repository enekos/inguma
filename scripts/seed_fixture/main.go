// Command seed_fixture writes two versions of @foo/bar into a corpus and
// artifact store for use by the track-A end-to-end smoke.
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"

	"github.com/enekos/inguma/internal/artifacts"
	"github.com/enekos/inguma/internal/corpus"
	"github.com/enekos/inguma/internal/manifest"
)

func main() {
	corpusDir := flag.String("corpus", "", "corpus directory")
	artifactsDir := flag.String("artifacts", "", "artifacts directory")
	flag.Parse()
	if *corpusDir == "" || *artifactsDir == "" {
		log.Fatal("need -corpus and -artifacts")
	}
	store := artifacts.NewFSStore(*artifactsDir)
	for _, v := range []string{"v1.0.0", "v1.1.0"} {
		tool := manifest.Tool{
			Name:        "@foo/bar",
			DisplayName: "Bar",
			Description: "fixture tool",
			Readme:      "README.md",
			License:     "MIT",
			Kind:        manifest.KindMCP,
			MCP:         &manifest.MCPConfig{Transport: "stdio", Command: "true"},
			Compatibility: manifest.Compatibility{
				Harnesses: []string{"claude-code"},
				Platforms: []string{"darwin", "linux"},
			},
		}
		mfJSON, err := json.MarshalIndent(tool, "", "  ")
		if err != nil {
			log.Fatal(err)
		}
		var buf bytes.Buffer
		if err := artifacts.Build(&buf, artifacts.Input{
			Owner: "foo", Slug: "bar", Version: v,
			Manifest: mfJSON, Readme: []byte("# bar"), License: []byte("MIT"),
		}); err != nil {
			log.Fatal(err)
		}
		sha, err := store.Put(artifacts.Ref{Owner: "foo", Slug: "bar", Version: v}, &buf)
		if err != nil {
			log.Fatal(err)
		}
		if err := corpus.WriteVersion(*corpusDir, corpus.VersionedEntry{
			Owner: "foo", Slug: "bar", Version: v,
			ManifestJSON: mfJSON,
			IndexMD:      []byte("---\nslug: bar\n---\n# bar"),
			ArtifactSHA:  sha,
		}); err != nil {
			log.Fatal(err)
		}
	}
	log.Println("seeded v1.0.0 and v1.1.0 of @foo/bar")
}
