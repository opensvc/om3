package resvol

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/volsignal"
	"golang.org/x/sys/unix"
)

type (
	// Reference is an element of the Configs and Secrets T field.
	// This type exists to host the parsing functions.
	Reference string

	// Metadata is the result of a Reference parsing.
	Metadata struct {
		ToPath    string
		FromKey   string
		FromStore naming.Path
	}

	// SigRoute is a relation between a signal number and the id of a resource supporting signaling
	SigRoute struct {
		Signum syscall.Signal
		RID    string
	}
)

func (t Metadata) IsEmpty() bool {
	return t.ToPath == "" && t.FromKey == ""
}

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

func (t T) getMetadata() []Metadata {
	l := make([]Metadata, 0)
	l = append(l, t.getMetadataByKind(naming.KindSec)...)
	l = append(l, t.getMetadataByKind(naming.KindCfg)...)
	return l
}

func (t T) getMetadataByKind(kind naming.Kind) []Metadata {
	l := make([]Metadata, 0)
	refs := t.getRefsByKind(kind)
	if len(refs) == 0 {
		// avoid the Head() call when possible
		return []Metadata{}
	}
	head := t.Head()
	if head == "" {
		return []Metadata{}
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
		if k == "" || md.FromKey == k {
			return true
		}
	}
	return false
}

func (t T) parseReference(s string, filter naming.Kind, head string) Metadata {
	if head == "" {
		return Metadata{}
	}

	// s = "sec/s1/k[12]:/here/"
	l := strings.SplitN(s, ":", 2)
	if len(l) != 2 {
		return Metadata{}
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
			return Metadata{}
		}
	case strings.HasPrefix(from, "sec/"):
		kind = naming.KindSec
		from = from[4:]
		if filter == naming.KindCfg {
			return Metadata{}
		}
	case strings.HasPrefix(from, "cfg/"):
		kind = naming.KindCfg
		from = from[4:]
		if filter == naming.KindSec {
			return Metadata{}
		}
	}
	// kind = path.KindSec
	// from = s1/k[12]

	l = strings.SplitN(from, "/", 2)
	if len(l) != 2 {
		return Metadata{}
	}
	if p, err := naming.NewPath(t.Path.Namespace, kind, l[0]); err != nil {
		return Metadata{}
	} else {
		return Metadata{
			ToPath:    toPath, // /here
			FromKey:   l[1],   // k[12]
			FromStore: p,      // <volns>/sec/s1
		}
	}
}

func (t *T) statusData() {
	for _, md := range t.getMetadata() {
		if !md.FromStore.Exists() {
			t.StatusLog().Warn("store %s does not exist: key %s data can not be installed in the volume", md.FromStore, md.FromKey)
			continue
		}
		keystore, err := object.NewKeystore(md.FromStore, object.WithVolatile(true))
		if err != nil {
			t.StatusLog().Warn("store %s init error: %s", md.FromStore, err)
			continue
		}
		matches, err := keystore.MatchingKeys(md.FromKey)
		if err != nil {
			t.StatusLog().Error("store %s keymatch %s: %s", md.FromStore, md.FromKey, err)
			continue
		}
		if len(matches) == 0 {
			t.StatusLog().Warn("store %s has no keys matching %s: data can not be installed in the volume", md.FromStore, md.FromKey)
		}
	}
}

func (t T) installData() error {
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
		return t.SendSignals()
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

func (t T) SendSignals() error {
	type signaler interface {
		SignalResource(string, syscall.Signal) error
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
		if err := o.SignalResource(sd.RID, sd.Signum); err != nil {
			return err
		}
		t.Log().Debug().Msgf("resource %s has been sent a signal %s", sd.RID, unix.SignalName(sd.Signum))
	}
	return nil
}

func (t T) InstallDataByKind(filter naming.Kind) (bool, error) {
	var changed bool

	for _, md := range t.getMetadataByKind(filter) {
		if !md.FromStore.Exists() {
			t.Log().Warn().Msgf("store %s does not exist: key %s data can not be installed in the volume", md.FromStore, md.FromKey)
			continue
		}
		keystore, err := object.NewKeystore(md.FromStore, object.WithVolatile(true))
		if err != nil {
			t.Log().Warn().Msgf("store %s init error: %s", md.FromStore, err)
		}
		var matches []string
		matches, err = keystore.MatchingKeys(md.FromKey)
		if err != nil {
			t.Log().Warn().Msgf("store %s keymatch %s: %s", md.FromStore, md.FromKey, err)
			continue
		}
		if len(matches) == 0 {
			t.Log().Warn().Msgf("store %s has no keys matching %s: data can not be installed in the volume", md.FromStore, md.FromKey)
			continue
		}
		for _, k := range matches {
			if err = keystore.InstallKeyTo(k, md.ToPath, t.Perm, t.DirPerm, t.User, t.Group); err != nil {
				return changed, err
			}
			changed = true
		}
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

func (t T) installDir(dir string, head string, mode *os.FileMode) error {
	if head == "" {
		return fmt.Errorf("refuse to install dir %s in /", dir)
	}
	p := filepath.Join(head, dir)
	var perm os.FileMode
	if mode == nil {
		perm = os.ModePerm
	} else {
		perm = *t.DirPerm
	}
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
	for _, dir := range t.Directories {
		if err := t.installDir(dir, head, t.DirPerm); err != nil {
			return err
		}
	}
	return nil
}
