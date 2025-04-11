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

	"github.com/opensvc/om3/core/driver"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/status"
	"github.com/opensvc/om3/util/file"
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
		ToHead        string
		ToPath        string
		FromPattern   string
		FromStore     naming.Path
		AccessControl KVInstallAccessControl
	}
	KVInstallAccessControl struct {
		User  string
		Group string
		Perm  *os.FileMode

		// always align dir access
		DirUser  string
		DirGroup string
		DirPerm  *os.FileMode

		// only align dir access on makedir
		MakedirUser  string
		MakedirGroup string
		MakedirPerm  *os.FileMode
	}
)

const (
	vKeyFile vKeyType = iota
	vKeyDir
)

var (
	ErrKeyNotFound = errors.New("key not found")
)

func (t KVInstall) IsEmpty() bool {
	return t.ToPath == "" && t.FromPattern == ""
}

func (t *kvStore) resolveKey(k string) ([]vKey, error) {
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

func (t *kvStore) _install(k string, dst string) error {
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
		}
		if _, err := t.installKey(vk, opt); err != nil {
			return err
		}
	}
	return err
}

// keyPath returns the full path to host's file containing the key decoded data.
func (t *kvStore) keyPath(vk vKey, dst string) string {
	if strings.HasSuffix(dst, "/") {
		name := filepath.Base(strings.TrimRight(vk.Key, "/"))
		return filepath.Join(dst, name)
	}
	return dst
}

func (t *kvStore) installKey(vk vKey, opt KVInstall) (bool, error) {
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
func (t *kvStore) installFileKey(vk vKey, opt KVInstall) (bool, error) {
	if strings.Contains(opt.ToPath, "..") {
		// paranoid checks before RemoveAll() and Remove()
		return false, fmt.Errorf("install file key not allowed: %s contains \"..\"", opt.ToPath)
	}
	b, err := t.decode(vk.Key)
	if err != nil {
		return false, err
	}
	if v, err := file.ExistsAndDir(opt.ToPath); err != nil {
		t.Log().Errorf("install key %s directory at location %s: %s", vk.Key, opt.ToPath, err)
	} else if v {
		t.Log().Infof("remove key %s directory at location %s", vk.Key, opt.ToPath)
		if err := os.RemoveAll(opt.ToPath); err != nil {
			return false, err
		}
	}
	vdir := filepath.Dir(opt.ToPath)
	info, err := os.Stat(vdir)
	switch {
	case os.IsNotExist(err):
		t.Log().Infof("create directory %s to host key %s", vdir, vk.Key)
		if err := t.makedir(vdir, opt.AccessControl); err != nil {
			return false, err
		}
	case file.IsNotDir(err):
	case err != nil:
		return false, err
	case info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0:
		t.Log().Infof("remove key %s file at parent location %s", vk.Key, vdir)
		if err := os.Remove(vdir); err != nil {
			return false, err
		}
	}
	return t.writeKey(vk, opt.ToPath, b, opt.AccessControl.Perm, opt.AccessControl.User, opt.AccessControl.Group)
}

// installDirKey creates a directory to host projected keys
func (t *kvStore) installDirKey(vk vKey, opt KVInstall) (bool, error) {
	if strings.HasSuffix(opt.ToPath, "/") {
		dirname := filepath.Base(vk.Key)
		opt.ToPath = filepath.Join(opt.ToPath, dirname) + "/"
	}
	if err := t.makedir(opt.ToPath, opt.AccessControl); err != nil {
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

func (t *kvStore) chmod(p string, mode *os.FileMode, info os.FileInfo) error {
	if mode == nil {
		return nil
	}
	if info != nil {
		if *mode == info.Mode().Perm() {
			return nil
		}
		t.Log().Infof("change %s permissions from %s to %s", p, info.Mode().Perm(), mode)
	} else {
		t.Log().Debugf("set %s permissions to %s", p, mode)
	}
	return os.Chmod(p, *mode)
}

func (t *kvStore) chown(p string, usr, grp string, info os.FileInfo) error {
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
				t.Log().Infof("change %s owner from %d:%d to %d:%d", p, currentUID, currentGID, uid, gid)
				return os.Chown(p, uid, gid)
			} else {
				return nil
			}
		}
	} else if uid > 0 || gid > 0 {
		t.Log().Debugf("set %s owner to %d:%d", p, uid, gid)
		return os.Chown(p, uid, gid)
	}
	return nil
}

// writeKey reads the r Reader and writes the byte stream to the file at dst.
// This function return false if the dst content didn't change.
func (t *kvStore) writeKey(vk vKey, dst string, b []byte, mode *os.FileMode, usr, grp string) (bool, error) {
	mtime := t.configModTime()
	info, err := os.Stat(dst)
	if errors.Is(err, os.ErrNotExist) {
		perm := os.ModePerm
		if mode != nil {
			perm = *mode
		}
		t.log.Infof("install key %s to %s with owner %s:%s perm %v", vk.Key, dst, usr, grp, perm)
		if err := os.WriteFile(dst, b, perm); err != nil {
			return true, err
		}
		if err := t.chown(dst, usr, grp, nil); err != nil {
			return true, err
		}
		return true, os.Chtimes(dst, mtime, mtime)
	} else if err != nil {
		return false, err
	}
	if err := t.chmod(dst, mode, info); err != nil {
		return false, err
	}
	if err := t.chown(dst, usr, grp, info); err != nil {
		return false, err
	}
	if mtime == file.ModTime(dst) {
		return false, nil
	}
	targetMD5 := md5.New().Sum(b)
	currentMD5, err := file.MD5(dst)
	if err != nil {
		return false, err
	}
	if string(currentMD5) == string(targetMD5) {
		t.log.Debugf("%s from key %s already installed and same md5: set access and modification times to %s", dst, vk.Key, mtime)
		return false, os.Chtimes(dst, mtime, mtime)
	}
	return false, nil
}

func (t *kvStore) InstallKey(keyName string) error {
	return t.postInstall(keyName)
}

func (t *kvStore) makedir(path string, opt KVInstallAccessControl) error {
	info, err := os.Stat(path)
	if err == nil {
		if err := t.chmod(path, opt.DirPerm, info); err != nil {
			return err
		}
		if err := t.chown(path, opt.DirUser, opt.DirGroup, info); err != nil {
			return err
		}
		return nil
	} else {
		t.log.Infof("install dir %s with owner %s:%s perm %v", path, opt.MakedirUser, opt.MakedirGroup, *opt.MakedirPerm)
		if err := os.MkdirAll(path, *opt.MakedirPerm); err != nil {
			return err
		}
		if err := t.chown(path, opt.MakedirUser, opt.MakedirGroup, nil); err != nil {
			return err
		}
	}
	return nil
}

func (t *kvStore) makedirs(opt KVInstall) error {
	if opt.ToHead == "" || !strings.HasSuffix(opt.ToPath, "/") {
		return nil
	}
	relPath := strings.TrimPrefix(opt.ToPath, opt.ToHead)
	for _, dir := range pathChain(relPath) {
		if err := t.makedir(filepath.Join(opt.ToHead, dir), opt.AccessControl); err != nil {
			return err
		}
	}
	return nil
}

func (t *kvStore) InstallKeyTo(opt KVInstall) error {
	t.log.Debugf("install key %s to %s", opt.FromPattern, opt.ToPath)
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
			return fmt.Errorf("install key %s to %s: %w", vk.Key, t.path, err)
		}
	}
	return nil
}

func (t *kvStore) postInstall(k string) error {
	changedVolumes := make(map[naming.Path]interface{})
	type resvoler interface {
		InstallDataByKind(naming.Kind) (bool, error)
		HasMetadata(p naming.Path, k string) bool
		Volume() (Vol, error)
		SendSignals(context.Context) error
	}
	paths, err := naming.InstalledPaths()
	if err != nil {
		return err
	}
	for _, p := range paths {
		if p.Namespace != t.path.Namespace {
			continue
		}
		if p.Kind != naming.KindSvc {
			continue
		}
		o, err := NewCore(p, WithVolatile(true))
		if err != nil {
			return err
		}
		for _, r := range resourcesByDrivergroups(o, []driver.Group{driver.GroupVolume}) {
			var i interface{} = r
			v := i.(resvoler)
			if !v.HasMetadata(t.path, k) {
				continue
			}
			vol, err := v.Volume()
			if err != nil {
				t.log.Warnf("post install %s %s: %s", p, r.RID(), err)
				continue
			}
			ctx := context.Background()
			st, err := vol.Status(ctx)
			if err != nil {
				t.log.Warnf("post install %s %s: %s", p, r.RID(), err)
				continue
			}
			if st.Avail != status.Up {
				continue
			}
			changed, err := v.InstallDataByKind(t.path.Kind)
			if err != nil {
				return err
			}
			if changed {
				changedVolumes[vol.Path()] = nil
			}
			if _, ok := changedVolumes[vol.Path()]; !ok {
				continue
			}
			t.log.Debugf("signal key %s referrer: %s (%s)", k, p, r.RID())
			if err := v.SendSignals(ctx); err != nil {
				t.log.Warnf("post install %s %s: %s", p, r.RID(), err)
				continue
			}
		}
	}
	return nil
}
