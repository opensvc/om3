package object

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/danwakefield/fnmatch"

	"github.com/opensvc/om3/v3/core/driver"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/volsignal"
	"github.com/opensvc/om3/v3/util/confined"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/plog"
)

type (
	vKeyType int

	vKey struct {
		Key  string
		Type vKeyType
		Keys []vKey
	}

	KVInstall struct {
		Required      bool
		ToLog         *plog.Logger
		ToHead        string
		ToPath        string
		FromPattern   string
		FromStore     naming.Path
		Source        string
		IsTemplate    bool
		AccessControl KVInstallAccessControl
		Signals       *volsignal.T

		// fs is where the install writes: the tree of ToHead, which the
		// install cannot leave, or the node for an install naming no head.
		fs confined.FS
	}
	KVInstallAccessControl struct {
		User  string
		Group string
		Perm  os.FileMode

		// always align dir access
		DirUser  string
		DirGroup string
		DirPerm  os.FileMode

		// only align dir access on makedir
		MakedirUser  string
		MakedirGroup string
		MakedirPerm  os.FileMode
	}
)

const (
	vKeyFile vKeyType = iota
	vKeyDir
)

var (
	ErrKeyNotFound = errors.New("key not found")
)

func (t KVInstall) RelativeToPath() string {
	relativePath, _ := strings.CutPrefix(t.ToPath, t.ToHead)
	return relativePath
}

func (t KVInstall) String() string {
	return fmt.Sprintf("%#v", t)
}

func (t KVInstall) IsZero() bool {
	return t.ToPath == "" && t.FromPattern == ""
}

func (t *dataStore) resolveKey(k string) ([]vKey, error) {
	var (
		dirs, keys []string
		err        error
		recurse    func(string) []vKey
	)
	if dirs, err = t.AllDirs(); err != nil {
		return []vKey{}, err
	}
	if keys, err = t.AllKeys(); err != nil {
		return []vKey{}, err
	}
	done := make(map[string]any)

	recurse = func(k string) []vKey {
		data := make([]vKey, 0)
		for _, p := range dirs {
			if p != k && !fnmatch.Match(k, p, fnmatch.FNM_PATHNAME) {
				continue
			}
			vks := recurse(p + "/*")
			data = append(data, vKey{
				Key:  p,
				Type: vKeyDir,
				Keys: vks,
			})
		}
		for _, p := range keys {
			if p != k && !fnmatch.Match(k, p, fnmatch.FNM_PATHNAME) {
				continue
			}
			if _, ok := done[p]; ok {
				continue
			}
			done[p] = nil
			data = append(data, vKey{
				Key:  p,
				Type: vKeyFile,
			})
		}
		return data
	}

	return recurse(k), nil
}

func mergeMapsets(m1 map[string]interface{}, m2 map[string]interface{}) map[string]interface{} {
	for k := range m1 {
		m2[k] = nil
	}
	return m2
}

func (t *dataStore) _install(k string, dst string) error {
	keys, err := t.resolveKey(k)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return fmt.Errorf("%s key %s not found", t.path, k)
	}
	for _, vk := range keys {
		opt := KVInstall{
			ToPath: dst,
			fs:     confined.Host(),
		}
		if _, err := t.installKey(vk, opt); err != nil {
			return err
		}
	}
	return err
}

// keyPath returns the full path to host's file containing the key decoded data.
func (t *dataStore) keyPath(vk vKey, dst string) string {
	if strings.HasSuffix(dst, "/") {
		name := filepath.Base(strings.TrimRight(vk.Key, "/"))
		return filepath.Join(dst, name)
	}
	return dst
}

func (t *dataStore) installKey(vk vKey, opt KVInstall) (bool, error) {
	switch vk.Type {
	case vKeyFile:
		opt.ToPath = t.keyPath(vk, opt.ToPath)
		return t.installFileKey(vk, opt)
	case vKeyDir:
		return t.installDirKey(vk, opt)
	default:
		return false, nil
	}
}

// installFileKey installs a key content in the host storage
func (t *dataStore) installFileKey(vk vKey, opt KVInstall) (bool, error) {
	if strings.Contains(opt.ToPath, "..") {
		// paranoid checks before RemoveAll() and Remove()
		return false, fmt.Errorf("install file key not allowed: %s contains \"..\"", opt.ToPath)
	}
	b, err := t.decode(vk.Key)
	if err != nil {
		return false, err
	}
	if info, err := opt.fs.Lstat(opt.ToPath); err == nil && info.IsDir() {
		opt.ToLog.Infof("remove key %s directory at location %s", vk.Key, opt.ToPath)
		if err := opt.fs.RemoveAll(opt.ToPath); err != nil {
			return false, err
		}
	}
	vdir := filepath.Dir(opt.ToPath)
	// The parent is read without following a link: a link there is replaced
	// by the directory it stands for, not written through.
	info, err := opt.fs.Lstat(vdir)
	switch {
	case os.IsNotExist(err):
		opt.ToLog.Infof("create directory %s to host key %s", vdir, vk.Key)
		if err := t.makedir(vdir, opt.AccessControl, opt.fs, opt.ToLog); err != nil {
			return false, err
		}
	case file.IsNotDir(err):
	case err != nil:
		return false, err
	case info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0:
		opt.ToLog.Infof("remove key %s file at parent location %s", vk.Key, vdir)
		if err := opt.fs.Remove(vdir); err != nil {
			return false, err
		}
		if err := t.makedir(vdir, opt.AccessControl, opt.fs, opt.ToLog); err != nil {
			return false, err
		}
	}
	return t.writeKey(vk, b, opt)
}

// installDirKey creates a directory to host projected keys
func (t *dataStore) installDirKey(vk vKey, opt KVInstall) (bool, error) {
	if strings.HasSuffix(opt.ToPath, "/") {
		dirname := filepath.Base(vk.Key)
		opt.ToPath = filepath.Join(opt.ToPath, dirname) + "/"
	}
	if err := t.makedir(opt.ToPath, opt.AccessControl, opt.fs, opt.ToLog); err != nil {
		return false, err
	}
	changed := false
	for _, k := range vk.Keys {
		v, err := t.installKey(k, opt)
		if err != nil {
			return changed, err
		}
		changed = changed || v
	}
	return changed, nil
}

func (t *dataStore) chmod(p string, perm os.FileMode, info os.FileInfo, fs confined.FS, log *plog.Logger) error {
	if info != nil {
		if perm == info.Mode().Perm() {
			return nil
		}
		log.Infof("change %s permissions from %s to %s", p, info.Mode().Perm(), perm)
	} else {
		log.Tracef("set %s permissions to %s", p, perm)
	}
	return fs.Chmod(p, perm)
}

// chown changes the owner of p, never of what a link at p leads to.
func (t *dataStore) chown(p string, usr, grp string, info os.FileInfo, fs confined.FS, log *plog.Logger) error {
	var uid, gid int
	if usr != "" {
		if i, err := strconv.Atoi(usr); err == nil {
			uid = i
		} else if u, err := user.Lookup(usr); err == nil {
			uid, _ = strconv.Atoi(u.Uid)
		} else {
			return fmt.Errorf("user %s is not numeric and not resolved", usr)
		}
	} else {
		uid = -1
	}
	if grp != "" {
		if i, err := strconv.Atoi(grp); err == nil {
			gid = i
		} else if g, err := user.LookupGroup(grp); err == nil {
			gid, _ = strconv.Atoi(g.Gid)
		} else {
			return fmt.Errorf("group %s is not numeric and not resolved", grp)
		}
	} else {
		gid = -1
	}
	if info != nil {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			currentUID := int(stat.Uid)
			currentGID := int(stat.Gid)
			if uid < 0 {
				uid = currentUID
			}
			if gid < 0 {
				gid = currentGID
			}
			if uid != currentUID || gid != currentGID {
				log.Infof("change %s owner from %d:%d to %d:%d", p, currentUID, currentGID, uid, gid)
				return fs.Lchown(p, uid, gid)
			} else {
				return nil
			}
		}
	} else if uid > 0 || gid > 0 {
		log.Tracef("set %s owner to %d:%d", p, uid, gid)
		return fs.Lchown(p, uid, gid)
	}
	return nil
}

// writeKey reads the r Reader and writes the byte stream to the file at dst.
// This function return false if the dst content didn't change.
//
// A link at dst is removed and the file written in its place: an install
// writes a file, and following the link would write where the link says,
// which whoever writes in the tree decides.
func (t *dataStore) writeKey(vk vKey, b []byte, opt KVInstall) (bool, error) {
	dst := opt.ToPath
	perm := opt.AccessControl.Perm
	usr := opt.AccessControl.User
	grp := opt.AccessControl.Group
	mtime := t.configModTime()
	info, err := opt.fs.Lstat(dst)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		opt.ToLog.Infof("remove the link at %s to install key %s", dst, vk.Key)
		if err := opt.fs.Remove(dst); err != nil {
			return false, err
		}
		info, err = opt.fs.Lstat(dst)
	}

	if errors.Is(err, os.ErrNotExist) {
		opt.ToLog.Infof("install key %s from %s to %s with owner %s:%s perm %v", vk.Key, t.path, dst, usr, grp, perm)
		if err := opt.fs.WriteFile(dst, b, perm); err != nil {
			return true, err
		}
		if err := t.chown(dst, usr, grp, nil, opt.fs, opt.ToLog); err != nil {
			return true, err
		}
		return true, opt.fs.Chtimes(dst, mtime, mtime)
	} else if err != nil {
		return false, err
	}
	if err := t.chmod(dst, perm, info, opt.fs, opt.ToLog); err != nil {
		return false, err
	}
	if err := t.chown(dst, usr, grp, info, opt.fs, opt.ToLog); err != nil {
		return false, err
	}
	if mtime == info.ModTime() {
		return false, nil
	}
	current, err := opt.fs.ReadFile(dst)
	if err != nil {
		return false, err
	}
	if md5.Sum(current) == md5.Sum(b) {
		opt.ToLog.Tracef("%s from key %s already installed and same md5: set access and modification times to %s", dst, vk.Key, mtime)
		return false, opt.fs.Chtimes(dst, mtime, mtime)
	}
	if err := opt.fs.WriteFile(dst, b, info.Mode()); err != nil {
		return true, err
	}
	opt.ToLog.Infof("reinstall key %s from %s to %s with owner %s:%s perm %v", vk.Key, t.path, dst, usr, grp, perm)
	return false, nil
}

func (t *dataStore) InstallKey(keyName string) error {
	return t.postInstall(keyName)
}

func (t *dataStore) makedir(path string, opt KVInstallAccessControl, fs confined.FS, log *plog.Logger) error {
	info, err := fs.Stat(path)
	if err == nil {
		if err := t.chmod(path, opt.DirPerm, info, fs, log); err != nil {
			return err
		}
		if err := t.chown(path, opt.DirUser, opt.DirGroup, info, fs, log); err != nil {
			return err
		}
		return nil
	} else {
		log.Infof("install dir %s with owner %s:%s perm %v", path, opt.MakedirUser, opt.MakedirGroup, opt.MakedirPerm)
		if err := fs.MkdirAll(path, opt.MakedirPerm); err != nil {
			return err
		}
		if err := t.chown(path, opt.MakedirUser, opt.MakedirGroup, nil, fs, log); err != nil {
			return err
		}
	}
	return nil
}

func (t *dataStore) makedirs(opt KVInstall) error {
	if opt.ToHead == "" || !strings.HasSuffix(opt.ToPath, "/") {
		return nil
	}
	relPath := strings.TrimPrefix(opt.ToPath, opt.ToHead)
	for _, dir := range pathChain(relPath) {
		if err := t.makedir(filepath.Join(opt.ToHead, dir), opt.AccessControl, opt.fs, opt.ToLog); err != nil {
			return err
		}
	}
	return nil
}

func (t *dataStore) InstallKeyTo(opt KVInstall) error {
	if opt.ToLog == nil {
		opt.ToLog = t.log
	}
	opt.ToLog.Tracef("install key %s to %s", opt.FromPattern, opt.ToPath)
	// An install into a head, the one of a volume, stays in it: the head is
	// written by the containers mounting the volume too, and a link they
	// plant there must not lead a write of the agent out of it. An install
	// naming no head writes where only the agent writes.
	if opt.ToHead != "" {
		tree, err := confined.Open(opt.ToHead)
		if err != nil {
			return fmt.Errorf("install key %s: %w", opt.FromPattern, err)
		}
		defer func() { _ = tree.Close() }()
		opt.fs = tree
	} else {
		opt.fs = confined.Host()
	}
	keys, err := t.resolveKey(opt.FromPattern)
	if err != nil {
		return fmt.Errorf("resolve %s key %s: %w", t.path, opt.FromPattern, err)
	}
	if len(keys) == 0 {
		if opt.Required {
			return fmt.Errorf("resolve %s key %s: %w", t.path, opt.FromPattern, ErrKeyNotFound)
		} else {
			return nil
		}
	}
	if err := t.makedirs(opt); err != nil {
		return err
	}
	for _, vk := range keys {
		if _, err := t.installKey(vk, opt); err != nil {
			return fmt.Errorf("install %s key %s to %s: %w", t.path, vk.Key, opt.ToPath, err)
		}
	}
	return nil
}

func (t *dataStore) postInstall(k string) error {
	type receiver interface {
		CanInstall(context.Context) (bool, error)
		InstallFromDatastore(context.Context, DataStore) (bool, error)
		InstallDataByKind(naming.Kind) (bool, error)
		HasMetadata(naming.Path, string) bool
		OldSendSignals(context.Context) error
	}
	ctx := context.Background()
	paths, err := naming.InstalledPaths()
	if err != nil {
		return err
	}
	for _, p := range paths {
		if !t.Allow(p.Namespace) {
			continue
		}
		if p.Kind != naming.KindSvc {
			continue
		}
		o, err := NewCore(p, WithVolatile(true))
		if err != nil {
			return err
		}
		var onChange func(context.Context) error
		for _, r := range resourcesByDrivergroups(o, []driver.Group{driver.GroupVolume, driver.GroupFS}) {
			// A resource that failed to configure has none of the state its
			// methods read, so it is skipped here as it is everywhere else a
			// resource is acted upon. Its configuration error is what its
			// status reports.
			if err := r.GetConfigurationError(); err != nil {
				t.log.Tracef("skip %s of %s: configuration error: %s", r.RID(), p, err)
				continue
			}
			receiverResource, ok := any(r).(receiver)
			if !ok {
				continue
			}
			if !receiverResource.HasMetadata(t.path, k) {
				continue
			}
			if ok, err := receiverResource.CanInstall(ctx); err != nil {
				return err
			} else if !ok {
				continue
			}

			if v, err := receiverResource.InstallFromDatastore(ctx, t); err != nil {
				return err
			} else if v {
				onChange = receiverResource.OldSendSignals
			}

			if v, err := receiverResource.InstallDataByKind(t.path.Kind); err != nil {
				return err
			} else if v {
				onChange = receiverResource.OldSendSignals
			}
		}
		if onChange != nil {
			t.log.Tracef("signal key %s referrer: %s", k, p)
			if err := onChange(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}
