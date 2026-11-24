package storeplakar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kballard/go-shellquote"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/executable"
)

const (
	dirPerm = 0700
)

type (
	// dirs are the private directories of the store on this node: the
	// plakar configuration (stores.yml) under the run directory, the cache
	// and the packages under var.
	dirs struct {
		config, cache, data string
	}
)

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

// keyMeta returns the key the option designates, written from <datastore>
// key <key>: a secret is never written in the section. A relative datastore
// of a store declared in a sec is of its namespace, "from ." naming the sec
// itself.
func (t *T) keyMeta(option string) (datarecv.KeyMeta, error) {
	s := t.Get(option)
	if s == "" {
		return datarecv.KeyMeta{}, fmt.Errorf("no %s: set it to from <sec> key <key>", option)
	}
	var km datarecv.KeyMeta
	var err error
	if owner := t.Owner(); !owner.IsZero() {
		km, err = datarecv.ParseKeyMetaRelSelf(s, owner)
	} else {
		km, err = datarecv.ParseKeyMetaRel(s, naming.NsSys)
	}
	if err != nil {
		return datarecv.KeyMeta{}, fmt.Errorf("%s: %w: write it from <sec> key <key>", option, err)
	}
	return km, nil
}

// decodeKey returns the content of the key. The key of a store declared in
// a sec is read as the object reading the store, whose namespace the
// datastore must share with. The key of a store of the cluster configuration
// is read as root.
func (t *T) decodeKey(option string, km datarecv.KeyMeta) ([]byte, error) {
	var b []byte
	var err error
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

// checkPassphrase refuses a passphrase plakar cannot read from the one line
// passphrase_cmd prints: empty, or holding a line break other than a
// trailing newline. The error never holds the passphrase.
func checkPassphrase(b []byte) error {
	b = bytes.TrimSuffix(b, []byte("\n"))
	switch {
	case len(b) == 0:
		return fmt.Errorf("passphrase: the key is empty")
	case bytes.ContainsAny(b, "\r\n"):
		return fmt.Errorf("passphrase: the key holds a line break: plakar reads one line")
	}
	return nil
}

// passphraseCmd is the command plakar runs through /bin/sh to read the
// passphrase: om printing the key, so the passphrase goes through a pipe
// only.
func passphraseCmd(km datarecv.KeyMeta) (string, error) {
	om, err := executable.Path()
	if err != nil {
		return "", err
	}
	return shellquote.Join(om, km.Path.String(), "key", "decode", km.Key), nil
}

// storesYML is what plakar store add writes for one store and its
// passphrase_cmd.
func (t *T) storesYML(location string) ([]byte, error) {
	km, err := t.keyMeta("passphrase")
	if err != nil {
		return nil, err
	}
	cmd, err := passphraseCmd(km)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(struct {
		Version string                       `yaml:"version"`
		Stores  map[string]map[string]string `yaml:"stores"`
	}{
		Version: "v1.0.0",
		Stores: map[string]map[string]string{
			strings.TrimPrefix(alias, "@"): {
				"location":       location,
				"passphrase_cmd": cmd,
			},
		},
	})
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
// directories and the plakar configuration when missing or stale, and the
// store itself when it is a local path not created yet. SetConfig has no
// error path, so every call runs it first.
//
// The passphrase is decoded and checked, then dropped: plakar reads it
// through passphrase_cmd.
//
// plakar empties stores.yml when it misses sources.yml or destinations.yml
// of its configuration directory, so the three are written.
func (t *T) ensure(ctx context.Context) error {
	location, dir, err := t.location()
	if err != nil {
		return err
	}
	km, err := t.keyMeta("passphrase")
	if err != nil {
		return err
	}
	passphrase, err := t.decodeKey("passphrase", km)
	if err != nil {
		return err
	}
	if err := checkPassphrase(passphrase); err != nil {
		return err
	}
	stores, err := t.storesYML(location)
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
		"stores.yml":       stores,
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
	cmd, err := t.commandAt(ctx, zerolog.InfoLevel, nil, "create")
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
