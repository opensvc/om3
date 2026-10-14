package datarecv

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

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

	// fileStamps are what each file an install line held was at its last
	// run, by path relative to the head: a file of a key gone is removed
	// only while it is the one installed, not one written in its place
	// since, as by hand once its removal failed.
	fileStamps map[string]fileStamp

	fileStamp struct {
		Size    int64     `json:"size"`
		ModTime time.Time `json:"mtime"`
	}
)

func newFileStamp(info os.FileInfo) fileStamp {
	return fileStamp{Size: info.Size(), ModTime: info.ModTime().UTC()}
}

// matches says whether the file is the one the stamp was taken of.
func (t fileStamp) matches(info os.FileInfo) bool {
	return t.Size == info.Size() && t.ModTime.Equal(info.ModTime())
}

func stampsFile(installFilesFile string) string {
	return installFilesFile + ".stamps"
}

func readFileStamps(filename string) fileStamps {
	m := make(fileStamps)
	if b, err := os.ReadFile(filename); err == nil {
		_ = json.Unmarshal(b, &m)
	}
	return m
}

func writeFileStamps(filename string, m fileStamps) error {
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

// stampHeld records what the files the line holds are, as installed.
func (t *DataRecv) stampHeld(head string, held []string, stamps fileStamps) {
	if len(held) == 0 || head == "" {
		return
	}
	tree, err := confined.Open(head)
	if err != nil {
		return
	}
	defer func() { _ = tree.Close() }()
	for _, p := range held {
		info, err := tree.Lstat(filepath.Join(head, filepath.Clean("/"+p)))
		if err != nil || info.IsDir() {
			continue
		}
		stamps[p] = newFileStamp(info)
	}
}

// installLineID names an install line: the keys it installs, and where. The
// item of a datastore of a line naming them by a pattern is the line, and the
// datastore: the line holds the files of each, and leaves the ones of the
// others to their item.
func installLineID(md object.KVInstall) string {
	if md.Line != "" {
		return md.Line + "@" + md.FromStore.String()
	}
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
	stamps := readFileStamps(stampsFile(filename))
	held = slices.Clone(held)
	slices.Sort(held)
	stale := make([]string, 0)
	for _, p := range m[id] {
		if !slices.Contains(held, p) && !m.heldByOther(id, p) {
			stale = append(stale, p)
		}
	}
	removed, failed := t.removeInstalled(head, stale, stamps)
	t.stampHeld(head, held, stamps)
	for _, p := range stale {
		if !slices.Contains(failed, p) {
			delete(stamps, p)
		}
	}
	if err := writeFileStamps(stampsFile(filename), stamps); err != nil {
		t.to.Log().Warnf("record the files install %s holds: %s", id, err)
	}
	// A file whose removal failed, as on a volume read-only for a while,
	// stays recorded, so a later run removes it.
	held = append(held, failed...)
	slices.Sort(held)
	held = slices.Compact(held)
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
// longer configured, as a line taken away from the install keyword, one
// installing elsewhere now, or the item of a datastore gone of a line naming
// them by a pattern, and returns the ones removed, by line.
func (t *DataRecv) pruneInstallLines(head string, ids []string) map[string][]string {
	filename := t.installFilesFile()
	if filename == "" {
		return nil
	}
	m := readInstallFiles(filename)
	stamps := readFileStamps(stampsFile(filename))
	removed := make(map[string][]string)
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
		l, failed := t.removeInstalled(head, stale, stamps)
		if len(l) > 0 {
			removed[id] = l
		}
		for _, p := range stale {
			if !slices.Contains(failed, p) {
				delete(stamps, p)
			}
		}
		if len(failed) > 0 {
			// Recorded still, for a later run to remove them.
			m[id] = failed
		}
	}
	if dropped {
		if err := writeInstallFiles(filename, m); err != nil {
			t.to.Log().Warnf("record the files the install lines hold: %s", err)
		}
		if err := writeFileStamps(stampsFile(filename), stamps); err != nil {
			t.to.Log().Warnf("record the files the install lines hold: %s", err)
		}
	}
	return removed
}

// removeInstalled removes the files at the paths, relative to head, an
// install line held and holds no more, and returns the ones removed, and the
// ones it failed to remove, which are to be tried again. A file that is not
// the one installed anymore, as stamps say, is left where it is: it was
// written in its place since. So is a file stamps say nothing of.
func (t *DataRecv) removeInstalled(head string, paths []string, stamps fileStamps) (removed, failed []string) {
	if len(paths) == 0 || head == "" {
		return nil, nil
	}
	tree, err := confined.Open(head)
	if err != nil {
		t.to.Log().Warnf("remove the files of the keys gone: %s", err)
		return nil, slices.Clone(paths)
	}
	defer func() { _ = tree.Close() }()
	for _, p := range paths {
		abs := filepath.Join(head, filepath.Clean("/"+p))
		info, err := tree.Lstat(abs)
		switch {
		case errors.Is(err, os.ErrNotExist):
			continue
		case err != nil:
			t.to.Log().Warnf("remove %s: %s", abs, err)
			failed = append(failed, p)
			continue
		case info.IsDir():
			continue
		}
		if stamp, ok := stamps[p]; !ok {
			t.to.Log().Warnf("leave %s: nothing recorded tells it is the file installed from a key now gone", abs)
			continue
		} else if !stamp.matches(info) {
			t.to.Log().Warnf("leave %s: it changed since it was installed from a key now gone", abs)
			continue
		}
		if err := tree.Remove(abs); err != nil {
			t.to.Log().Warnf("remove %s: %s", abs, err)
			failed = append(failed, p)
			continue
		}
		t.to.Log().Infof("remove %s: the key it was installed from is gone", abs)
		removed = append(removed, p)
	}
	return removed, failed
}
