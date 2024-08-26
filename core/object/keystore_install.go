package object

import (
	"context"
	"crypto/md5"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

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
		Path string
		Keys []vKey
	}
)

const (
	vKeyFile vKeyType = iota
	vKeyDir
)

func (t *keystore) resolveKey(k string) ([]vKey, error) {
	var (
		dirs, keys []string
		err        error
	)
	if dirs, err = t.AllDirs(); err != nil {
		return []vKey{}, err
	}
	if keys, err = t.AllKeys(); err != nil {
		return []vKey{}, err
	}
	data, _ := resolveKeyRecurse(k, make(map[string]interface{}), dirs, keys)
	return data, nil
}

func mergeMapsets(m1 map[string]interface{}, m2 map[string]interface{}) map[string]interface{} {
	for k := range m1 {
		m2[k] = nil
	}
	return m2
}

func resolveKeyRecurse(k string, done map[string]interface{}, dirs, keys []string) ([]vKey, map[string]interface{}) {
	data := make([]vKey, 0)
	for _, p := range dirs {
		if p != k && !fnmatch.Match(p, k, fnmatch.FNM_PATHNAME|fnmatch.FNM_LEADING_DIR) {
			continue
		}
		vks, rdone := resolveKeyRecurse(p+"/*", done, dirs, keys)
		done = mergeMapsets(done, rdone)
		data = append(data, vKey{
			Key:  k,
			Type: vKeyDir,
			Path: p,
			Keys: vks,
		})
	}
	for _, p := range keys {
		if p != k && !fnmatch.Match(p, k, fnmatch.FNM_PATHNAME|fnmatch.FNM_LEADING_DIR) {
			continue
		}
		if _, ok := done[p]; ok {
			continue
		}
		done[p] = nil
		data = append(data, vKey{
			Key:  k,
			Type: vKeyFile,
			Path: p,
		})
	}
	return data, done
}

func (t *keystore) _install(k string, dst string) error {
	keys, err := t.resolveKey(k)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return fmt.Errorf("%s key=%s not found", t.path, k)
	}
	for _, vk := range keys {
		if _, err := t.installKey(vk, dst, nil, nil, "", ""); err != nil {
			return err
		}
	}
	return err
}

// keyPath returns the full path to host's file containing the key decoded data.
func (t *keystore) keyPath(vk vKey, dst string) string {
	if strings.HasSuffix(dst, "/") {
		name := filepath.Base(strings.TrimRight(vk.Path, "/"))
		return filepath.Join(dst, name)
	}
	return dst
}

func (t *keystore) installKey(vk vKey, dst string, mode *os.FileMode, dirmode *os.FileMode, usr, grp string) (bool, error) {
	switch vk.Type {
	case vKeyFile:
		vpath := t.keyPath(vk, dst)
		return t.installFileKey(vk, vpath, mode, dirmode, usr, grp)
	case vKeyDir:
		return t.installDirKey(vk, dst, mode, dirmode, usr, grp)
	default:
		return false, nil
	}
}

// installFileKey installs a key content in the host storage
func (t *keystore) installFileKey(vk vKey, dst string, mode *os.FileMode, dirmode *os.FileMode, usr, grp string) (bool, error) {
	if strings.Contains(dst, "..") {
		// paranoid checks before RemoveAll() and Remove()
		return false, fmt.Errorf("install file key not allowed: %s contains \"..\"", dst)
	}
	b, err := t.decode(vk.Key)
	if err != nil {
		return false, err
	}
	if v, err := file.ExistsAndDir(dst); err != nil {
		t.Log().Errorf("install %s key=%s directory at location %s: %s", t.path, vk.Key, dst, err)
	} else if v {
		t.Log().Infof("remove %s key=%s directory at location %s", t.path, vk.Key, dst)
		if err := os.RemoveAll(dst); err != nil {
			return false, err
		}
	}
	vdir := filepath.Dir(dst)
	info, err := os.Stat(vdir)
	switch {
	case os.IsNotExist(err):
		t.Log().Infof("create directory %s to host %s key=%s", vdir, t.path, vk.Key)
		if err := os.MkdirAll(vdir, *dirmode); err != nil {
			return false, err
		}
	case file.IsNotDir(err):
	case err != nil:
		return false, err
	case info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0:
		t.Log().Infof("remove %s key=%s file at parent location %s", t.path, vk.Key, vdir)
		if err := os.Remove(vdir); err != nil {
			return false, err
		}
	}
	return t.writeKey(vk, dst, b, mode, usr, grp)
}

// installDirKey creates a directory to host projected keys
func (t *keystore) installDirKey(vk vKey, dst string, mode *os.FileMode, dirmode *os.FileMode, usr, grp string) (bool, error) {
	if strings.HasSuffix(dst, "/") {
		dirname := filepath.Base(vk.Path)
		dst = filepath.Join(dst, dirname, "")
	}
	if err := os.MkdirAll(dst, *dirmode); err != nil {
		return false, err
	}
	changed := false
	for _, k := range vk.Keys {
		v, err := t.installKey(k, dst, mode, dirmode, usr, grp)
		if err != nil {
			return changed, err
		}
		changed = changed || v
	}
	return changed, nil
}

func (t *keystore) chmod(p string, mode *os.FileMode) error {
	if mode == nil {
		return nil
	}
	return os.Chmod(p, *mode)
}

func (t *keystore) chown(p string, usr, grp string) error {
	uid := -1
	gid := -1
	if usr != "" {
		if i, err := strconv.Atoi(usr); err == nil {
			uid = i
		} else if u, err := user.Lookup(usr); err == nil {
			uid, _ = strconv.Atoi(u.Uid)
		} else {
			return fmt.Errorf("user %s is not integer and not resolved", usr)
		}
	}
	if grp != "" {
		if i, err := strconv.Atoi(grp); err == nil {
			gid = i
		} else if g, err := user.LookupGroup(grp); err == nil {
			gid, _ = strconv.Atoi(g.Gid)
		} else {
			return fmt.Errorf("group %s is not integer and not resolved", grp)
		}
	}
	return os.Chown(p, uid, gid)
}

// writeKey reads the r Reader and writes the byte stream to the file at dst.
// This function return false if the dst content didn't change.
func (t *keystore) writeKey(vk vKey, dst string, b []byte, mode *os.FileMode, usr, grp string) (bool, error) {
	mtime := t.configModTime()
	if file.Exists(dst) {
		if err := t.chmod(dst, mode); err != nil {
			return false, err
		}
		if err := t.chown(dst, usr, grp); err != nil {
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
			t.log.Debugf("%s/%s in %s already installed and same md5: set access and modification times to %s", t.path.Name, vk.Key, dst, mtime)
			return false, os.Chtimes(dst, mtime, mtime)
		}
	}
	t.log.Infof("install %s/%s in %s", t.path.Name, vk.Key, dst)
	perm := os.ModePerm
	if mode != nil {
		perm = *mode
	}
	if err := os.WriteFile(dst, b, perm); err != nil {
		return true, err
	}
	if err := t.chown(dst, usr, grp); err != nil {
		return false, err
	}
	return true, os.Chtimes(dst, mtime, mtime)
}

func (t *keystore) InstallKey(keyName string) error {
	return t.postInstall(keyName)
}

func (t *keystore) InstallKeyTo(keyName string, dst string, mode *os.FileMode, dirmode *os.FileMode, usr, grp string) error {
	t.log.Debugf("install %s key %s to %s", t.path, keyName, dst)
	keys, err := t.resolveKey(keyName)
	if err != nil {
		return fmt.Errorf("resolve %s key %s: %w", t.path, keyName, err)
	}
	if len(keys) == 0 {
		return fmt.Errorf("resolve %s key %s: no key found", t.path, keyName)
	}
	for _, vk := range keys {
		if _, err := t.installKey(vk, dst, mode, dirmode, usr, grp); err != nil {
			return fmt.Errorf("install key %s at path %s: %w", vk.Key, t.path, err)
		}
	}
	return nil
}

func (t *keystore) postInstall(k string) error {
	changedVolumes := make(map[naming.Path]interface{})
	type resvoler interface {
		InstallDataByKind(naming.Kind) (bool, error)
		HasMetadata(p naming.Path, k string) bool
		Volume() (Vol, error)
		SendSignals() error
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
			t.log.Debugf("signal %s %s referrer: %s (%s)", t.path, k, p, r.RID())
			if err := v.SendSignals(); err != nil {
				t.log.Warnf("post install %s %s: %s", p, r.RID(), err)
				continue
			}
		}
	}
	return nil
}
