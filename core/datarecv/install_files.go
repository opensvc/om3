package datarecv

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/util/confined"
)

type (
	// installFiles are the files each install line of a resource held at
	// its last run, relative to the head, by line.
	//
	// A key the store no longer holds installs nothing, and leaves the file
	// it installed until then: the record tells the line it held it, and
	// the file is removed, as what the line installs.
	installFiles map[string][]string
)

// installLineID names an install line: the keys it installs, and where.
func installLineID(md object.KVInstall) string {
	return md.FromStore.String() + ":" + md.FromPattern + ":" + strings.TrimPrefix(md.ToPath, md.ToHead)
}

// installFilesFile returns the file recording the files the install lines of
// the resource held, none when the receiver is not a resource of an object.
func (t *DataRecv) installFilesFile() string {
	o, ok := t.to.GetObject().(object.Core)
	if !ok {
		return ""
	}
	r, ok := t.to.(ridder)
	if !ok {
		return ""
	}
	return filepath.Join(o.Path().VarDir(), r.RID(), "install_files")
}

func readInstallFiles(filename string) installFiles {
	m := make(installFiles)
	if filename == "" {
		return m
	}
	b, err := os.ReadFile(filename)
	if err != nil {
		return m
	}
	_ = json.Unmarshal(b, &m)
	return m
}

func writeInstallFiles(filename string, m installFiles) error {
	if len(m) == 0 {
		if err := os.Remove(filename); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		return err
	}
	tmp := filename + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, filename)
}

// pruneInstallLine removes the files the line id held at its last run and
// holds no more, records the ones it holds, and returns the ones removed.
//
// It runs after a run of the line that went through, which held what its
// keys install: a run that failed, or found no store, says nothing of the
// keys, and its files are left as they are.
func (t *DataRecv) pruneInstallLine(head, id string, held []string) []string {
	filename := t.installFilesFile()
	if filename == "" {
		return nil
	}
	m := readInstallFiles(filename)
	held = slices.Clone(held)
	slices.Sort(held)
	stale := make([]string, 0)
	for _, p := range m[id] {
		if !slices.Contains(held, p) && !m.heldByOther(id, p) {
			stale = append(stale, p)
		}
	}
	removed := t.removeInstalled(head, stale)
	if !slices.Equal(m[id], held) {
		if len(held) == 0 {
			delete(m, id)
		} else {
			m[id] = held
		}
		if err := writeInstallFiles(filename, m); err != nil {
			t.to.Log().Warnf("record the files install %s holds: %s", id, err)
		}
	}
	return removed
}

// heldByOther says whether a line other than id holds the file p: two lines
// installing the same file, the one whose key is gone leaves it to the other.
func (m installFiles) heldByOther(id, p string) bool {
	for other, files := range m {
		if other != id && slices.Contains(files, p) {
			return true
		}
	}
	return false
}

// pruneInstallLines removes the files of the install lines recorded and no
// longer configured, as a line taken away from the install keyword or one
// installing elsewhere now, and returns the ones removed.
func (t *DataRecv) pruneInstallLines(head string, ids []string) []string {
	filename := t.installFilesFile()
	if filename == "" {
		return nil
	}
	m := readInstallFiles(filename)
	removed := make([]string, 0)
	dropped := false
	for id, files := range m {
		if slices.Contains(ids, id) {
			continue
		}
		delete(m, id)
		dropped = true
		stale := make([]string, 0, len(files))
		for _, p := range files {
			if !m.heldByOther(id, p) {
				stale = append(stale, p)
			}
		}
		removed = append(removed, t.removeInstalled(head, stale)...)
	}
	if dropped {
		if err := writeInstallFiles(filename, m); err != nil {
			t.to.Log().Warnf("record the files the install lines hold: %s", err)
		}
	}
	return removed
}

// removeInstalled removes the files at the paths, relative to head, an
// install line held and holds no more, and returns the ones removed.
//
// The head is written by what mounts it too: a removal stays in it, removes
// a file or a link, never what a link leads to, and leaves a directory found
// there, which the line did not install.
func (t *DataRecv) removeInstalled(head string, paths []string) []string {
	if len(paths) == 0 || head == "" {
		return nil
	}
	tree, err := confined.Open(head)
	if err != nil {
		t.to.Log().Warnf("remove the files of the keys gone: %s", err)
		return nil
	}
	defer func() { _ = tree.Close() }()
	removed := make([]string, 0)
	for _, p := range paths {
		abs := filepath.Join(head, filepath.Clean("/"+p))
		info, err := tree.Lstat(abs)
		switch {
		case errors.Is(err, os.ErrNotExist):
			continue
		case err != nil:
			t.to.Log().Warnf("remove %s: %s", abs, err)
			continue
		case info.IsDir():
			continue
		}
		if err := tree.Remove(abs); err != nil {
			t.to.Log().Warnf("remove %s: %s", abs, err)
			continue
		}
		t.to.Log().Infof("remove %s: the key it was installed from is gone", abs)
		removed = append(removed, p)
	}
	return removed
}
