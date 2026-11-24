package storeplakar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

const (
	dirPerm = 0700
)

type (
	// dirs are the private directories of the store on this node: the
	// plakar configuration (stores.yml) and the passphrase under the run
	// directory, the cache and the packages under var.
	dirs struct {
		config, cache, data string
	}
)

// keyfile is the file holding the passphrase, read whole by plakar.
func (t dirs) keyfile() string {
	return filepath.Join(t.config, "passphrase")
}

// name returns the section declaring the store, without its prefix.
func (t *T) name() string {
	return strings.TrimPrefix(t.Key("").Section, backup.SectionPrefix)
}

// dirs returns the directories of the store: node/<name> for a store of the
// cluster configuration, sec/<namespace>/<sec>/<name> for a store of a sec,
// so two stores of the same name never share their directories.
func (t *T) dirs() dirs {
	rel := filepath.Join("node", t.name())
	if owner := t.Owner(); !owner.IsZero() {
		rel = filepath.Join("sec", owner.Namespace, owner.Name, t.name())
	}
	return dirs{
		config: filepath.Join(rawconfig.Paths.Run, "store", "plakar", rel),
		cache:  filepath.Join(rawconfig.Paths.Var, "plakar", rel, "cache"),
		data:   filepath.Join(rawconfig.Paths.Var, "plakar", rel, "data"),
	}
}

// location returns the store location plakar understands, a local path
// becoming a fs:// url, and the directory of a store of the node, empty for
// a remote store.
func (t *T) location() (string, string, error) {
	s := t.Get("url")
	switch {
	case s == "":
		return "", "", fmt.Errorf("no url")
	case strings.HasPrefix(s, "/"):
		return "fs://" + s, s, nil
	case strings.HasPrefix(s, "fs://"):
		return s, strings.TrimPrefix(s, "fs://"), nil
	default:
		return s, "", nil
	}
}

// IsLocal says whether the store is a directory of the node.
func (t *T) IsLocal() bool {
	_, dir, _ := t.location()
	return dir != ""
}

// decodeKey returns the content of the key the option designates, written
// from <datastore> key <key>: a secret is never written in the section.
//
// The key of a store declared in a sec is read as the object reading the
// store, whose namespace the datastore must share with, "from ." naming the
// sec itself. The key of a store of the cluster configuration is read as
// root.
func (t *T) decodeKey(option string) ([]byte, error) {
	s := t.Get(option)
	if s == "" {
		return nil, fmt.Errorf("no %s: set it to from <sec> key <key>", option)
	}
	var km datarecv.KeyMeta
	var err error
	if owner := t.Owner(); !owner.IsZero() {
		km, err = datarecv.ParseKeyMetaRelSelf(s, owner)
	} else {
		km, err = datarecv.ParseKeyMetaRel(s, naming.NsSys)
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w: write it from <sec> key <key>", option, err)
	}
	var b []byte
	if reader := t.Reader(); reader != "" {
		km.From = reader
		b, err = km.Decode()
	} else {
		b, err = km.RootDecode()
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", option, err)
	}
	return b, nil
}

// storesYML is what plakar store add writes for one store.
func storesYML(location string) []byte {
	return []byte("version: v1.0.0\nstores:\n    " + strings.TrimPrefix(alias, "@") + ":\n        location: " + location + "\n")
}

// writeFile writes b as the file name of dir, in place once complete.
func writeFile(dir, name string, b []byte) error {
	f, err := os.CreateTemp(dir, "."+name+".*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, name))
}

// mkdirPrivate makes path 0700 whatever the umask, and whether it existed
// or not: MkdirAll applies the umask and leaves an existing directory as is.
func mkdirPrivate(path string) error {
	if err := os.MkdirAll(path, dirPerm); err != nil {
		return err
	}
	return os.Chmod(path, dirPerm)
}

// ensure prepares the node for the store before a call: the private
// directories, the passphrase and the plakar configuration when missing or
// stale, and the store itself when it is a local path not created yet.
// SetConfig has no error path, so every call runs it first.
//
// plakar empties stores.yml when it misses sources.yml or destinations.yml
// of its configuration directory, so the three are written.
func (t *T) ensure(ctx context.Context) error {
	location, dir, err := t.location()
	if err != nil {
		return err
	}
	passphrase, err := t.decodeKey("passphrase")
	if err != nil {
		return err
	}
	d := t.dirs()
	for _, path := range []string{d.config, d.cache, d.data} {
		if err := mkdirPrivate(path); err != nil {
			return err
		}
	}
	for name, b := range map[string][]byte{
		"passphrase":       passphrase,
		"stores.yml":       storesYML(location),
		"sources.yml":      []byte("version: v1.0.0\nsources: {}\n"),
		"destinations.yml": []byte("version: v1.0.0\ndestinations: {}\n"),
	} {
		if got, err := os.ReadFile(filepath.Join(d.config, name)); err == nil && bytes.Equal(got, b) {
			continue
		}
		if err := writeFile(d.config, name, b); err != nil {
			return err
		}
	}
	if dir == "" {
		// A remote store is never created by the driver.
		return nil
	}
	if v, err := storeExists(dir); err != nil {
		return err
	} else if v {
		return nil
	}
	t.Log().Infof("create the store at %s", location)
	cmd, err := t.commandAt(ctx, zerolog.InfoLevel, nil, location, "create")
	if err != nil {
		return err
	}
	if err := cmd.Run(); err != nil {
		return cmdError("create", cmd, err)
	}
	return nil
}

// storeExists tells whether a local store directory exists and holds
// something: an empty or missing directory is a store to create.
func storeExists(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return len(entries) > 0, nil
}
