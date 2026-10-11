package datarecv

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
)

type (
	// Installed is the record of the last install of a resource that
	// wrote files: the daemon watching it tells the tasks reading them.
	Installed struct {
		At    time.Time `json:"at"`
		Files []string  `json:"files"`
	}
)

// InstalledDir returns the directory holding the install records of the
// resources of the instance of p, a file named after each resource.
func InstalledDir(p naming.Path) string {
	return filepath.Join(p.VarDir(), "installed")
}

// ReadInstalled returns the install record of the file filename.
func ReadInstalled(filename string) (Installed, error) {
	var v Installed
	b, err := os.ReadFile(filename)
	if err != nil {
		return v, err
	}
	err = json.Unmarshal(b, &v)
	return v, err
}

// recordInstalled records the files an install wrote, relative to the head
// of the resource, and nothing when it wrote none: an install finding its
// files up to date tells nothing.
//
// The record is renamed in place, so a watcher of the directory sees it
// created once, whole. Failing to record leaves the files installed, and
// what reads them unaware: it is a warning, not an install error.
func (t *DataRecv) recordInstalled(files []string) {
	if len(files) == 0 {
		return
	}
	o, ok := t.to.GetObject().(object.Core)
	if !ok {
		t.to.Log().Warnf("record installed files: the receiver is not an object")
		return
	}
	r, ok := t.to.(ridder)
	if !ok {
		t.to.Log().Warnf("record installed files: the receiver is not a resource")
		return
	}
	if err := writeInstalled(InstalledDir(o.Path()), r.RID(), files); err != nil {
		t.to.Log().Warnf("record installed files: %s", err)
	}
}

// writeInstalled writes the record of files in dir, named rid.
func writeInstalled(dir, rid string, files []string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	b, err := json.Marshal(Installed{At: time.Now(), Files: files})
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, filepath.Join(dir, rid)); err != nil {
		return err
	}
	return nil
}
