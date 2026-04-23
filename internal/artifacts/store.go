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
