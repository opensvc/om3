package object

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/danwakefield/fnmatch"
	"github.com/pkg/errors"
	"opensvc.com/opensvc/core/drivergroup"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/status"
	"opensvc.com/opensvc/util/file"
)

type (
	OptsInstall struct {
		Global OptsGlobal
		Key    string `flag:"key"`
	}

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

func (t Keystore) resolveKey(k string) ([]vKey, error) {
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
	for k, _ := range m1 {
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

func (t Keystore) _install(k string, dst string) error {
	keys, err := t.resolveKey(k)
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return fmt.Errorf("%s key=%s not found", t, k)
	}
	for _, vk := range keys {
		if _, err := t.installKey(vk, dst, nil, nil, nil, nil); err != nil {
			return err
		}
	}
	return err
}

// keyPath returns the full path to host's file containing the key decoded data.
func (t Keystore) keyPath(vk vKey, dst string) string {
	if strings.HasSuffix(dst, "/") {
		name := filepath.Base(strings.TrimRight(vk.Path, "/"))
		return filepath.Join(dst, name)
	}
	return dst
}

func (t Keystore) installKey(vk vKey, dst string, mode *os.FileMode, dirmode *os.FileMode, usr *user.User, grp *user.Group) (bool, error) {
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
func (t Keystore) installFileKey(vk vKey, dst string, mode *os.FileMode, dirmode *os.FileMode, usr *user.User, grp *user.Group) (bool, error) {
	if strings.Contains(dst, "..") {
		// paranoid checks before RemoveAll() and Remove()
		return false, fmt.Errorf("install file key not allowed: %s contains \"..\"", dst)
	}
	b, err := t.decode(vk.Key)
	if err != nil {
		return false, err
	}
	if file.ExistsAndDir(dst) {
		t.Log().Info().Msgf("remove %s key=%s directory at location %s", t, vk.Key, dst)
		if err := os.RemoveAll(dst); err != nil {
			return false, err
		}
	}
	vdir := filepath.Dir(dst)
	if file.ExistsAndRegular(vdir) || file.ExistsAndSymlink(vdir) {
		t.Log().Info().Msgf("remove %s key=%s file at parent location %s", t, vk.Key, vdir)
		if err := os.Remove(vdir); err != nil {
			return false, err
		}
	}
	if !file.Exists(vdir) {
		t.Log().Info().Msgf("create directory %s to host %s key=%s", vdir, t, vk.Key)
		if err := os.MkdirAll(vdir, *dirmode); err != nil {
			return false, err
		}
	}
	return t.writeKey(vk, dst, b, mode, usr, grp)
}

// installDirKey creates a directory to host projected keys
func (t Keystore) installDirKey(vk vKey, dst string, mode *os.FileMode, dirmode *os.FileMode, usr *user.User, grp *user.Group) (bool, error) {
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

func (t Keystore) chmod(p string, mode *os.FileMode) error {
	if mode == nil {
		return nil
	}
	return os.Chmod(p, *mode)
}

func (t Keystore) chown(p string, usr *user.User, grp *user.Group) error {
	var err error
	uid := -1
	gid := -1
	if usr != nil {
		if uid, err = strconv.Atoi(usr.Uid); err != nil {
			return fmt.Errorf("uid %s is not integer", usr.Uid)
		}
	}
	if grp != nil {
		if gid, err = strconv.Atoi(grp.Gid); err != nil {
			return fmt.Errorf("gid %s is not integer", grp.Gid)
		}
	}
	return os.Chown(p, uid, gid)
}

// writeKey reads the r Reader and writes the byte stream to the file at dst.
// This function return false if the dst content didn't change.
func (t Keystore) writeKey(vk vKey, dst string, b []byte, mode *os.FileMode, usr *user.User, grp *user.Group) (bool, error) {
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
			t.log.Debug().Msgf("%s/%s in %s already installed and same md5: set access and modification times to %s", t.Path.Name, vk.Key, dst, mtime)
			return false, os.Chtimes(dst, mtime, mtime)
		}
	}
	t.log.Info().Msgf("install %s/%s in %s", t.Path.Name, vk.Key, dst)
	perm := os.ModePerm
	if mode != nil {
		perm = *mode
	}
	if err := ioutil.WriteFile(dst, b, perm); err != nil {
		return true, err
	}
	return true, os.Chtimes(dst, mtime, mtime)
}

func (t Keystore) Install(options OptsInstall) error {
	return t.postInstall(options.Key)
}

func (t Keystore) InstallKey(k string, dst string, mode *os.FileMode, dirmode *os.FileMode, usr *user.User, grp *user.Group) error {
	t.log.Debug().Msgf("install key=%s to %s", k, dst)
	keys, err := t.resolveKey(k)
	if err != nil {
		return errors.Wrapf(err, "%s", t.Path)
	}
	if len(keys) == 0 {
		return fmt.Errorf("%s key=%s not found", t.Path, k)
	}
	for _, vk := range keys {
		if _, err := t.installKey(vk, dst, mode, dirmode, usr, grp); err != nil {
			return errors.Wrapf(err, "%s: %s", t.Path, vk.Key)
		}
	}
	return nil
}

func (t Keystore) postInstall(k string) error {
	changedVolumes := make(map[path.T]interface{})
	sel := NewSelection(t.Path.Namespace+"/svc/*", SelectionWithLocal(true))
	type resvoler interface {
		InstallDataByKind(kind.T) (bool, error)
		HasMetadata(p path.T, k string) bool
		Volume() (*Vol, error)
		SendSignals() error
	}
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	for _, p := range paths {
		o, err := NewBaserFromPath(p, WithVolatile(true))
		if err != nil {
			return err
		}
		for _, r := range ResourcesByDrivergroups(o, []drivergroup.T{drivergroup.Volume}) {
			var i interface{} = r
			v := i.(resvoler)
			if !v.HasMetadata(t.Path, k) {
				continue
			}
			vol, err := v.Volume()
			if err != nil {
				t.log.Warn().Msgf("post install %s %s: %s", p, r.RID(), err)
				continue
			}
			st, err := vol.Status(OptsStatus{})
			if err != nil {
				t.log.Warn().Msgf("post install %s %s: %s", p, r.RID(), err)
				continue
			}
			if st.Avail != status.Up {
				continue
			}
			changed, err := v.InstallDataByKind(t.Path.Kind)
			if err != nil {
				return err
			}
			if changed {
				changedVolumes[vol.Path] = nil
			}
			if _, ok := changedVolumes[vol.Path]; !ok {
				continue
			}
			t.log.Debug().Msgf("signal %s %s referrer: %s (%s)", t.Path, k, p, r.RID())
			if err := v.SendSignals(); err != nil {
				t.log.Warn().Msgf("post install %s %s: %s", p, r.RID(), err)
				continue
			}
		}
	}
	return nil
}
