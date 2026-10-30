package compliance

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
)

const (
	// archiveRoot is the directory the archive of the modules holds them in.
	archiveRoot = "compliance"

	// upstreamDir is the directory of the upstream compliance objects, kept
	// through an update when the archive does not bring it.
	upstreamDir = "com.opensvc"
)

// Install replaces the installed modules by those of the gzipped tarball r.
//
// The archive is unpacked next to the modules, which it replaces once
// complete: a failure leaves them as they were. The previous modules are
// left in the ".bck" sibling of the modules directory.
func (t *T) Install(r io.Reader) error {
	parent := filepath.Dir(t.varDir)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(parent, filepath.Base(t.varDir)+".tmp-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := unpackModules(r, tmp); err != nil {
		return fmt.Errorf("unpack the compliance modules: %w", err)
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return err
	}
	return t.swapModules(tmp)
}

func (t *T) swapModules(dir string) error {
	backup := t.varDir + ".bck"
	if err := os.RemoveAll(backup); err != nil {
		return err
	}
	if err := os.Rename(t.varDir, backup); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(dir, t.varDir); err != nil {
		return errors.Join(err, os.Rename(backup, t.varDir))
	}
	upstream := filepath.Join(t.varDir, upstreamDir)
	if _, err := os.Lstat(upstream); !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.Rename(filepath.Join(backup, upstreamDir), upstream); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// unpackModules writes in dir the files the gzipped tarball r holds under
// archiveRoot, and fails on an archive holding none, which would leave the
// node without modules.
func unpackModules(r io.Reader, dir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()
	root, err := os.OpenRoot(dir)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	var count int
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
		// git archive records the commit in a global header.
		if hdr.Typeflag == tar.TypeXGlobalHeader {
			continue
		}
		rel, err := moduleRelPath(hdr.Name)
		if err != nil {
			return err
		}
		if rel == "." {
			continue
		}
		if err := unpackEntry(root, rel, hdr, tr); err != nil {
			return fmt.Errorf("%s: %w", hdr.Name, err)
		}
		count++
	}
	if count == 0 {
		return fmt.Errorf("no file under %s/ in the archive", archiveRoot)
	}
	return nil
}

// moduleRelPath returns the path under archiveRoot of an archive entry.
func moduleRelPath(name string) (string, error) {
	rel, err := filepath.Rel(archiveRoot, filepath.FromSlash(path.Clean(name)))
	if err != nil || !filepath.IsLocal(rel) {
		return "", fmt.Errorf("%s: not under %s/", name, archiveRoot)
	}
	return rel, nil
}

func unpackEntry(root *os.Root, rel string, hdr *tar.Header, r io.Reader) error {
	switch hdr.Typeflag {
	case tar.TypeDir:
		return root.MkdirAll(rel, 0o755)
	case tar.TypeReg:
		if err := root.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
			return err
		}
		f, err := root.OpenFile(rel, os.O_WRONLY|os.O_CREATE|os.O_EXCL, hdr.FileInfo().Mode().Perm())
		if err != nil {
			return err
		}
		if _, err := io.Copy(f, r); err != nil {
			_ = f.Close()
			return err
		}
		return f.Close()
	case tar.TypeSymlink:
		target := filepath.FromSlash(hdr.Linkname)
		if !filepath.IsLocal(filepath.Join(filepath.Dir(rel), target)) || filepath.IsAbs(target) {
			return fmt.Errorf("link to %s leaves the modules", hdr.Linkname)
		}
		if err := root.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
			return err
		}
		return root.Symlink(target, rel)
	default:
		return fmt.Errorf("unsupported entry type %q", string(hdr.Typeflag))
	}
}
