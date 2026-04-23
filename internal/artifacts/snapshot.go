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
	Manifest []byte
	Readme   []byte
	License  []byte
}

var epoch = time.Unix(0, 0).UTC()

func Build(w io.Writer, in Input) error {
	gz := gzip.NewWriter(w)
	gz.ModTime = epoch
	gz.Name = ""
	tw := tar.NewWriter(gz)

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
