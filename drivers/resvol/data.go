package resvol

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/volsignal"
)

type (
	// Reference is an element of the Configs and Secrets T field.
	// This type exists to host the parsing functions.
	Reference string

	// SigRoute is a relation between a signal number and the id of a resource supporting signaling
	SigRoute struct {
		Signum syscall.Signal
		RID    string
	}
)

func (t T) getRefs() []string {
	refs := make([]string, 0)
	refs = append(refs, t.getRefsByKind(naming.KindSec)...)
	refs = append(refs, t.getRefsByKind(naming.KindCfg)...)
	return refs
}

func (t T) getRefsByKind(filter naming.Kind) []string {
	refs := make([]string, 0)
	switch filter {
	case naming.KindSec:
		refs = append(refs, t.Secrets...)
	case naming.KindCfg:
		refs = append(refs, t.Configs...)
	}
	return refs
}

func (t T) getMetadata() []object.KVInstall {
	l := make([]object.KVInstall, 0)
	l = append(l, t.getMetadataByKind(naming.KindSec)...)
	l = append(l, t.getMetadataByKind(naming.KindCfg)...)
	return l
}

func (t T) getMetadataByKind(kind naming.Kind) []object.KVInstall {
	l := make([]object.KVInstall, 0)
	refs := t.getRefsByKind(kind)
	if len(refs) == 0 {
		// avoid the Head() call when possible
		return l
	}
	head := t.Head()
	if head == "" {
		return l
	}
	for _, ref := range refs {
		md := t.parseReference(ref, kind, head)
		if md.IsEmpty() {
			continue
		}
		l = append(l, md)
	}
	return l
}

// HasMetadata returns true if the volume has a configs or secrets reference to
// <namespace>/<kind>/<name>[/<key>]
func (t T) HasMetadata(p naming.Path, k string) bool {
	for _, md := range t.getMetadataByKind(p.Kind) {
		if md.FromStore.Name != p.Name {
			continue
		}
		if k == "" || md.FromPattern == k {
			return true
		}
	}
	return false
}

func (t T) parseReference(s string, filter naming.Kind, head string) object.KVInstall {
	if head == "" {
		return object.KVInstall{}
	}

	// s = "sec/s1/k[12]:/here/"
	l := strings.SplitN(s, ":", 2)
	if len(l) != 2 {
		return object.KVInstall{}
	}
	toPath := filepath.Join(head, l[1])
	// toPath = "/here"

	if strings.HasSuffix(l[1], "/") {
		// BEWARE: do not drop the trailing /, as it's the directory marker
		//         filepath.Join() drops it, so let's restore it.
		toPath = toPath + "/"
	}

	from := strings.TrimLeft(l[0], "/")
	// from = "sec/s1/k[12]"

	kind := filter

	switch {
	case strings.HasPrefix(from, "usr/"):
		kind = naming.KindUsr
		from = from[4:]
		if filter == naming.KindCfg {
			return object.KVInstall{}
		}
	case strings.HasPrefix(from, "sec/"):
		kind = naming.KindSec
		from = from[4:]
		if filter == naming.KindCfg {
			return object.KVInstall{}
		}
	case strings.HasPrefix(from, "cfg/"):
		kind = naming.KindCfg
		from = from[4:]
		if filter == naming.KindSec {
			return object.KVInstall{}
		}
	}
	// kind = path.KindSec
	// from = s1/k[12]

	l = strings.SplitN(from, "/", 2)
	if len(l) != 2 {
		return object.KVInstall{}
	}
	if p, err := naming.NewPath(t.Path.Namespace, kind, l[0]); err != nil {
		return object.KVInstall{}
	} else {
		var perm *os.FileMode
		if t.Perm == nil {
			switch kind {
			case naming.KindSec:
				perm = &defaultSecPerm
			case naming.KindCfg:
				perm = &defaultCfgPerm
			}
		} else {
			perm = t.Perm
		}
		t.Log().Infof("install %s to %s with perm %v", toPath, head, perm)
		return object.KVInstall{
			ToPath:      toPath, // /here
			ToHead:      head,
			FromPattern: l[1], // k[12]
			FromStore:   p,    // <volns>/sec/s1
			AccessControl: object.KVInstallAccessControl{
				User:    t.User,
				Group:   t.Group,
				Perm:    perm,
				DirPerm: t.getDirPerm(),
			},
		}
	}
}

func (t *T) statusData() {
	for _, md := range t.getMetadata() {
		if !md.FromStore.Exists() {
			t.StatusLog().Warn("store %s does not exist: key %s data can not be installed in the volume", md.FromStore, md.FromPattern)
			continue
		}
		keystore, err := object.NewKeystore(md.FromStore, object.WithVolatile(true))
		if err != nil {
			t.StatusLog().Warn("store %s init error: %s", md.FromStore, err)
			continue
		}
		matches, err := keystore.MatchingKeys(md.FromPattern)
		if err != nil {
			t.StatusLog().Error("store %s keymatch %s: %s", md.FromStore, md.FromPattern, err)
			continue
		}
		if len(matches) == 0 {
			t.StatusLog().Warn("store %s has no keys matching %s: data can not be installed in the volume", md.FromStore, md.FromPattern)
		}
	}
}

func (t T) installData(ctx context.Context) error {
	changed := false
	if err := t.installDirs(); err != nil {
		return err
	}
	if v, err := t.InstallDataByKind(naming.KindSec); err != nil {
		return err
	} else {
		changed = v || changed
	}
	if v, err := t.InstallDataByKind(naming.KindCfg); err != nil {
		return err
	} else {
		changed = v || changed
	}
	if changed {
		return t.SendSignals(ctx)
	}
	return nil
}

func (t T) signalData() []SigRoute {
	routes := make([]SigRoute, 0)
	for i, ridmap := range volsignal.Parse(t.Signal) {
		for rid := range ridmap {
			routes = append(routes, SigRoute{
				Signum: i,
				RID:    rid,
			})
		}
	}
	return routes
}

func (t T) SendSignals(ctx context.Context) error {
	type signaler interface {
		SignalResource(context.Context, string, syscall.Signal) error
	}
	i, err := object.New(t.Path)
	if err != nil {
		return err
	}
	o, ok := i.(signaler)
	if !ok {
		return fmt.Errorf("%s does not implement SignalResource()", t.Path)
	}
	for _, sd := range t.signalData() {
		if err := o.SignalResource(ctx, sd.RID, sd.Signum); err != nil {
			return err
		}
		t.Log().Debugf("resource %s has been sent a signal %s", sd.RID, unix.SignalName(sd.Signum))
	}
	return nil
}

func (t T) InstallDataByKind(filter naming.Kind) (bool, error) {
	var changed bool

	for _, md := range t.getMetadataByKind(filter) {
		if !md.FromStore.Exists() {
			t.Log().Warnf("store %s does not exist: key %s data can not be installed in the volume", md.FromStore, md.FromPattern)
			continue
		}
		keystore, err := object.NewKeystore(md.FromStore, object.WithVolatile(true))
		if err != nil {
			t.Log().Warnf("store %s init error: %s", md.FromStore, err)
		}
		if err = keystore.InstallKeyTo(md); err != nil {
			return changed, err
		}
		changed = true
	}
	return changed, nil
}

func (t T) chmod(p string, mode *os.FileMode) error {
	if mode == nil {
		return nil
	}
	return os.Chmod(p, *mode)
}

func (t T) chown(p string, usr, grp string) error {
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

func (t T) installDir(dir string, head string, perm os.FileMode) error {
	if head == "" {
		return fmt.Errorf("refuse to install dir %s in /", dir)
	}
	p := filepath.Join(head, dir)
	info, err := os.Stat(p)
	switch {
	case os.IsNotExist(err):
		if err := os.MkdirAll(p, perm); err != nil {
			return err
		}
		if err := t.chown(p, t.User, t.Group); err != nil {
			return err
		}
	case err != nil:
		return err
	default:
		if !info.IsDir() {
			return fmt.Errorf("directory path %s is already occupied by a non-directory", p)
		}
		if err := t.chmod(p, &perm); err != nil {
			return err
		}
		if err := t.chown(p, t.User, t.Group); err != nil {
			return err
		}
	}
	return nil
}

func (t T) installDirs() error {
	if len(t.Directories) == 0 {
		return nil
	}
	head := t.Head()
	if head == "" {
		return fmt.Errorf("refuse to install dirs in empty (ie /) head")
	}
	dirPerm := t.getDirPerm()
	for _, dir := range t.Directories {
		if err := t.installDir(dir, head, *dirPerm); err != nil {
			return err
		}
	}
	return nil
}
