package storeplakar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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

// schemeRE matches the scheme opening a url, as plakar names its backends.
var schemeRE = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*://`)

// name returns the section declaring the store, without its prefix.
func (t *T) name() string {
	return strings.TrimPrefix(t.Key("").Section, backup.SectionPrefix)
}

// validName refuses a store name that is not one path element of its own:
// the name makes a directory of the private directories of the store, which
// a separator or a parent directory would move elsewhere on the node.
func validName(s string) error {
	switch {
	case s == "", s == ".", s == "..", strings.ContainsAny(s, "/\\\x00"):
		return fmt.Errorf("invalid store name %q: a store is named with one path element", s)
	}
	return nil
}

// dirs returns the directories of the store: node/<name> for a store of the
// cluster configuration, sec/<namespace>/<sec>/<name> for a store of a sec,
// so two stores of the same name never share their directories.
func (t *T) dirs() (dirs, error) {
	name := t.name()
	if err := validName(name); err != nil {
		return dirs{}, err
	}
	rel := filepath.Join("node", name)
	if owner := t.Owner(); !owner.IsZero() {
		rel = filepath.Join("sec", owner.Namespace, owner.Name, name)
	}
	return dirs{
		config: filepath.Join(rawconfig.Paths.Run, "store", "plakar", rel),
		cache:  filepath.Join(rawconfig.Paths.Var, "plakar", rel, "cache"),
		data:   filepath.Join(rawconfig.Paths.Var, "plakar", rel, "data"),
	}, nil
}

// location returns the store location plakar understands, a local path
// becoming a fs:// url, and the directory of a store of the node, empty for
// a remote store.
//
// A url is an absolute path, a fs:// url of one, or the <scheme>:// url of a
// location plakar reaches by itself. Anything else is refused: plakar reads
// a relative path from its working directory, the one of the daemon or of
// the command, a directory of the node no keyword named.
func (t *T) location() (string, string, error) {
	s := t.Get("url")
	switch {
	case s == "":
		return "", "", fmt.Errorf("no url")
	case filepath.IsAbs(s):
		dir := filepath.Clean(s)
		return "fs://" + dir, dir, nil
	case strings.HasPrefix(s, "fs://"):
		dir := strings.TrimPrefix(s, "fs://")
		if !filepath.IsAbs(dir) {
			return "", "", fmt.Errorf("url %q is not an absolute path", s)
		}
		dir = filepath.Clean(dir)
		return "fs://" + dir, dir, nil
	case schemeRE.MatchString(s):
		return s, "", nil
	default:
		return "", "", fmt.Errorf("url %q is neither an absolute path nor a <scheme>:// url", s)
	}
}

// IsLocal says whether the store is a directory of the node. A url the
// driver refuses is reported local: the answer guards the stores of a sec,
// which are never a directory of the node, and Localer has no error to say
// the url cannot be judged, so the guard refuses what it cannot tell remote.
func (t *T) IsLocal() bool {
	_, dir, err := t.location()
	return err != nil || dir != ""
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
// only. The key name follows a --, so a key named as a flag is read as a
// name.
func passphraseCmd(km datarecv.KeyMeta) (string, error) {
	om, err := executable.Path()
	if err != nil {
		return "", err
	}
	return shellquote.Join(om, km.Path.String(), "key", "decode", "--", km.Key), nil
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

// prepare makes the node ready to call plakar on the store: the private
// directories and the plakar configuration, written when missing or stale.
// SetConfig has no error path, so every call runs it first. It returns the
// directory of a store of the node, empty for a remote store.
//
// The passphrase is decoded and checked, then dropped: plakar reads it
// through passphrase_cmd.
//
// plakar empties stores.yml when it misses sources.yml or destinations.yml
// of its configuration directory, so the three are written.
func (t *T) prepare() (string, error) {
	location, dir, err := t.location()
	if err != nil {
		return "", err
	}
	d, err := t.dirs()
	if err != nil {
		return "", err
	}
	km, err := t.keyMeta("passphrase")
	if err != nil {
		return "", err
	}
	passphrase, err := t.decodeKey("passphrase", km)
	if err != nil {
		return "", err
	}
	if err := checkPassphrase(passphrase); err != nil {
		return "", err
	}
	stores, err := t.storesYML(location)
	if err != nil {
		return "", err
	}
	for _, path := range []string{d.config, d.cache, d.data} {
		if err := mkdirPrivate(path); err != nil {
			return "", err
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
			return "", err
		}
	}
	return dir, nil
}

// open prepares the node for a call reading the store, or deleting from it,
// and tells whether there is a store to call: a store of the node missing
// or empty holds no run, and is not created to say so. A status on a node
// whose shared mount is not mounted would otherwise create a new store in
// the mount point. A remote store is for plakar to find.
func (t *T) open() (bool, error) {
	dir, err := t.prepare()
	if err != nil {
		return false, err
	}
	if dir == "" {
		return true, nil
	}
	return storeExists(dir)
}

// ensure prepares the node for a call writing a run, and creates the store
// when it is a local path not created yet: only a write makes a store. A
// remote store is never created by the driver.
func (t *T) ensure(ctx context.Context) error {
	dir, err := t.prepare()
	if err != nil {
		return err
	}
	if dir == "" {
		return nil
	}
	if v, err := storeExists(dir); err != nil {
		return err
	} else if v {
		return nil
	}
	t.Log().Infof("create the store at %s", dir)
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
