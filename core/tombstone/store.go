package tombstone

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/opensvc/om3/v3/core/rawconfig"
)

// Dir returns the directory the records of the node are kept in.
func Dir() string {
	return filepath.Join(rawconfig.NodeVarDir(), "tombstone")
}

func recordFile(dir, id string) (string, error) {
	if id == "" || strings.ContainsAny(id, "/\\") || id == "." || id == ".." {
		return "", fmt.Errorf("invalid tombstone id %q", id)
	}
	return filepath.Join(dir, id+".json"), nil
}

// Save writes the record to its file in dir, atomically: written to a
// temporary file synced to stable storage, renamed in place, and the
// directory synced, so a crash leaves the former record or this one, never
// an empty one.
func Save(dir string, rec Record) error {
	p, err := recordFile(dir, rec.OrchestrationID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, "."+rec.OrchestrationID+".*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() { _ = os.Remove(tmp) }()
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		return err
	}
	return syncDir(dir)
}

func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = d.Close() }()
	return d.Sync()
}

// Remove removes the file of the record of id from dir. A record absent is
// no error.
func Remove(dir, id string) error {
	p, err := recordFile(dir, id)
	if err != nil {
		return err
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncDir(dir)
}

// Load returns the records kept in dir. A file that can not be read or
// parsed is skipped, the others loaded, and its error returned with them.
// A directory absent holds no record.
func Load(dir string) ([]Record, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	var (
		l    []Record
		errs []error
	)
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".json") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			errs = append(errs, err)
			continue
		}
		var rec Record
		if err := json.Unmarshal(b, &rec); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", name, err))
			continue
		}
		if rec.OrchestrationID+".json" != name {
			errs = append(errs, fmt.Errorf("%s: holds the record of %s", name, rec.OrchestrationID))
			continue
		}
		if rec.Holders == nil {
			rec.Holders = make(map[string]Holder)
		}
		l = append(l, rec)
	}
	return l, errors.Join(errs...)
}
