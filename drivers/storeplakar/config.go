package storeplakar

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"

	"github.com/opensvc/om3/v3/core/backup"
	"github.com/opensvc/om3/v3/core/datarecv"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/util/command"
	"github.com/opensvc/om3/v3/util/executable"
)

const (
	dirPerm = 0700
)

type (
	// dirs are the private directories of the store on this node: the
	// plakar configuration (stores.yml) and the credential files under the
	// run directory, a memory filesystem commands are not run from, the
	// cache, the ssh command of a sftp store and the packages under var.
	dirs struct {
		config, cache, bin, data string
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
// so two stores of the same name never share their configuration and cache.
// The data directory, where plakar finds its plugins, is the one plakar
// installs them in for the user om runs as: shared by the stores, so the
// backends an operator installed, as sftp or s3, are the stores' too.
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
		bin:    filepath.Join(rawconfig.Paths.Var, "plakar", rel, "bin"),
		data:   dataDir(),
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

// checkOneLine refuses a secret plakar cannot read as one line, the one
// passphrase_cmd prints or the value of an option of stores.yml: empty, or
// holding a line break other than a trailing newline. The error never holds
// the secret.
func checkOneLine(option string, b []byte) error {
	b = bytes.TrimSuffix(b, []byte("\n"))
	switch {
	case len(b) == 0:
		return fmt.Errorf("%s: the key is empty", option)
	case bytes.ContainsAny(b, "\r\n"):
		return fmt.Errorf("%s: the key holds a line break: plakar reads one line", option)
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

// storesYML is what plakar store add writes for one store, its
// passphrase_cmd and the options of its credentials.
func (t *T) storesYML(location string, options map[string]string) ([]byte, error) {
	km, err := t.keyMeta("passphrase")
	if err != nil {
		return nil, err
	}
	cmd, err := passphraseCmd(km)
	if err != nil {
		return nil, err
	}
	entry := map[string]string{
		"location":       location,
		"passphrase_cmd": cmd,
	}
	for k, v := range options {
		entry[k] = v
	}
	return yaml.Marshal(struct {
		Version string                       `yaml:"version"`
		Stores  map[string]map[string]string `yaml:"stores"`
	}{
		Version: "v1.0.0",
		Stores:  map[string]map[string]string{strings.TrimPrefix(alias, "@"): entry},
	})
}

// securityCheckMarker is the file of the cache directory of a store saying
// its plakar has its security check disabled. It lives beside the record
// plakar keeps of it, in the cache, so a cache removed takes both away and
// the check is disabled again, where a marker kept elsewhere would outlive
// the record and leave plakar checking its releases feed.
const securityCheckMarker = "om3.security-check-disabled"

// disableSecurityCheck has plakar stop checking its releases feed for
// security updates, which it does once a day by default from each of its
// configurations, as each store has its own: a node must not call out to the
// plakar site when it backs up. plakar records it in its cache directory,
// so it runs once per store and cache, as securityCheckMarker records. Replaced by
// the tests faking plakar.
var disableSecurityCheck = runDisableSecurityCheck

func runDisableSecurityCheck(d dirs) error {
	p := getPath("plakar")
	if p == "" {
		return errPlakarNotInstalled
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := command.New(
		command.WithName(p),
		command.WithArgs([]string{"-configdir", d.config, "-cachedir", d.cache, "-datadir", d.data, "-disable-security-check"}),
		command.WithContext(ctx),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
	)
	if err := cmd.Run(); err != nil {
		return cmdError("-disable-security-check", cmd, err)
	}
	return nil
}

// warmUp has plakar do, in a call whose output is dropped, what it does on
// the first call finding its cache directory not ready, and prints on its
// standard output: the packages of the data directory unpacked into the
// cache, a line per file, and its welcome. The call after a package was
// installed or upgraded, the cache removed or plakar upgraded would otherwise
// print them before a listing, which is then not read, or before the content
// of a member, which would be restored with them.
//
// plakar alone knows when it has to: no record of the driver, of the
// packages or of the cache, would follow every reason it has. So the call is
// made once per driver, a command or an action, before its first call:
// plakar version, which opens no store and derives no key, a few hundredths
// of a second against the second of each call opening the store. A call
// found printing these lines is not one to run again: a stream is read
// once, and the content of a member is not told from them.
//
// Replaced by the tests faking plakar.
var warmUp = runWarmUp

func runWarmUp(t *T, d dirs) error {
	p := getPath("plakar")
	if p == "" {
		return errPlakarNotInstalled
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	cmd := command.New(
		command.WithName(p),
		command.WithArgs([]string{"-configdir", d.config, "-cachedir", d.cache, "-datadir", d.data, "version"}),
		command.WithContext(ctx),
		command.WithEnv(t.env(d)),
		command.WithBufferedStdout(),
		command.WithBufferedStderr(),
	)
	if err := cmd.Run(); err != nil {
		return cmdError("version", cmd, err)
	}
	return nil
}

// warm runs warmUp before the first plakar call of the driver. A failure is
// logged and tried again by the next call: the call it precedes says what
// is wrong with plakar better than its version does.
func (t *T) warm(d dirs) {
	t.warmMu.Lock()
	defer t.warmMu.Unlock()
	if t.warmed {
		return
	}
	if err := warmUp(t, d); err != nil {
		t.Log().Warnf("warm the cache of plakar up: %s", err)
		return
	}
	t.warmed = true
}

// writeFile writes b as the file name of dir, in place once complete.
func writeFile(dir, name string, b []byte) error {
	return writeFileMode(dir, name, b, filePerm)
}

// writeFileMode writes b as the file name of dir, with the mode perm, in
// place once complete. The file is made readable by its owner only, and
// given its mode before it has its name.
func writeFileMode(dir, name string, b []byte, perm os.FileMode) error {
	f, err := os.CreateTemp(dir, "."+name+".*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err := f.Write(b); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
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
// dataDir returns the data directory of plakar, replaced by the tests.
var dataDir = defaultDataDir

// defaultDataDir returns the directory plakar pkg add installs the plugins
// in for the user om runs as, read from the home of that user rather than
// from the environment: the daemon, run by systemd, may have no HOME, and
// the commands and the daemon must find the same plugins. XDG_DATA_HOME is
// not read for the same reason. A user with no home uses a directory of
// the agent.
func defaultDataDir() string {
	if u, err := user.Current(); err == nil && u.HomeDir != "" {
		return filepath.Join(u.HomeDir, ".local", "share", "plakar")
	}
	return filepath.Join(rawconfig.Paths.Var, "plakar", "data")
}

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
// through passphrase_cmd. The credentials of a remote location are decoded
// into the options of stores.yml and the files of the configuration
// directory, and a location missing the ones of its scheme is refused here,
// before any plakar call.
//
// plakar empties stores.yml when it misses sources.yml or destinations.yml
// of its configuration directory, so the three are written.
//
// The first call of the driver has plakar ready its cache: see warmUp.
func (t *T) prepare() (string, error) {
	return t.configure(false)
}

// configure is prepare, writing stores.yml again when rewrite is set, even
// when it holds what it should: plakar said it could not resolve the store.
func (t *T) configure(rewrite bool) (string, error) {
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
	if err := checkOneLine("passphrase", passphrase); err != nil {
		return "", err
	}
	creds, err := t.credentials(location, d)
	if err != nil {
		return "", err
	}
	stores, err := t.storesYML(location, creds.options)
	if err != nil {
		return "", err
	}
	for _, path := range []string{d.config, d.cache} {
		if err := mkdirPrivate(path); err != nil {
			return "", err
		}
	}
	// The data directory is the user's: made when missing, its mode left
	// as the operator set it.
	if err := os.MkdirAll(d.data, dirPerm); err != nil {
		return "", err
	}
	if err := creds.install(d); err != nil {
		return "", err
	}
	for name, b := range map[string][]byte{
		"stores.yml":       stores,
		"sources.yml":      []byte("version: v1.0.0\nsources: {}\n"),
		"destinations.yml": []byte("version: v1.0.0\ndestinations: {}\n"),
	} {
		if upToDate(filepath.Join(d.config, name), b, filePerm) && !(rewrite && name == "stores.yml") {
			continue
		}
		if err := writeFile(d.config, name, b); err != nil {
			return "", err
		}
	}
	if _, err := os.Stat(filepath.Join(d.cache, securityCheckMarker)); errors.Is(err, os.ErrNotExist) {
		if err := disableSecurityCheck(d); err != nil {
			t.Log().Warnf("disable the security check of plakar: %s", err)
		} else if err := writeFile(d.cache, securityCheckMarker, nil); err != nil {
			t.Log().Warnf("record the security check of plakar disabled: %s", err)
		}
	}
	t.warm(d)
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
