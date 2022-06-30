package resvol

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
	"opensvc.com/opensvc/core/kind"
	"opensvc.com/opensvc/core/object"
	"opensvc.com/opensvc/core/path"
	"opensvc.com/opensvc/core/volsignal"
	"opensvc.com/opensvc/util/file"
)

type (
	// Reference is an element of the Configs and Secrets T field.
	// This type exists to host the parsing functions.
	Reference string

	// Metadata is the result of a Reference parsing.
	Metadata struct {
		ToPath    string
		FromKey   string
		FromStore path.T
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
	refs = append(refs, t.getRefsByKind(kind.Sec)...)
	refs = append(refs, t.getRefsByKind(kind.Cfg)...)
	return refs
}

func (t T) getRefsByKind(filter kind.T) []string {
	refs := make([]string, 0)
	switch filter {
	case kind.Sec:
		refs = append(refs, t.Secrets...)
	case kind.Cfg:
		refs = append(refs, t.Configs...)
	}
	return refs
}

func (t T) getMetadata() []Metadata {
	l := make([]Metadata, 0)
	l = append(l, t.getMetadataByKind(kind.Sec)...)
	l = append(l, t.getMetadataByKind(kind.Cfg)...)
	return l
}

func (t T) getMetadataByKind(kd kind.T) []Metadata {
	l := make([]Metadata, 0)
	refs := t.getRefsByKind(kd)
	if len(refs) == 0 {
		// avoid the Head() call when possible
		return []Metadata{}
	}
	head := t.Head()
	if head == "" {
		return []Metadata{}
	}
	for _, ref := range refs {
		md := t.parseReference(ref, kd, head)
		if md.IsEmpty() {
			continue
		}
		l = append(l, md)
	}
	return l
}

// HasMetadata returns true if the volume has a configs or secrets reference to
// <namespace>/<kind>/<name>[/<key>]
func (t T) HasMetadata(p path.T, k string) bool {
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

func (t T) parseReference(s string, filter kind.T, head string) Metadata {
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

	kd := filter

	switch {
	case strings.HasPrefix(from, "usr/"):
		kd = kind.Usr
		from = from[4:]
		if filter == kind.Cfg {
			return Metadata{}
		}
	case strings.HasPrefix(from, "sec/"):
		kd = kind.Sec
		from = from[4:]
		if filter == kind.Cfg {
			return Metadata{}
		}
	case strings.HasPrefix(from, "cfg/"):
		kd = kind.Cfg
		from = from[4:]
		if filter == kind.Sec {
			return Metadata{}
		}
	}
	// kd = kind.Sec
	// from = s1/k[12]

	l = strings.SplitN(from, "/", 2)
	if len(l) != 2 {
		return Metadata{}
	}
	if p, err := path.New(l[0], t.Path.Namespace, kd.String()); err != nil {
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
		if !object.Exists(md.FromStore) {
			t.StatusLog().Warn("store %s does not exist: key %s data can not be installed in the volume", md.FromStore, md.FromKey)
			continue
		}
		keystore, err := object.NewKeystoreFromPath(md.FromStore, object.WithVolatile(true))
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
	if v, err := t.InstallDataByKind(kind.Sec); err != nil {
		return err
	} else {
		changed = v || changed
	}
	if v, err := t.InstallDataByKind(kind.Cfg); err != nil {
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
		for rid, _ := range ridmap {
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
	i, err := object.NewFromPath(t.Path)
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

func (t T) InstallDataByKind(filter kind.T) (bool, error) {
	var changed bool

	for _, md := range t.getMetadataByKind(filter) {
		if !object.Exists(md.FromStore) {
			t.Log().Warn().Msgf("store %s does not exist: key %s data can not be installed in the volume", md.FromStore, md.FromKey)
			continue
		}
		keystore, err := object.NewKeystoreFromPath(md.FromStore, object.WithVolatile(true))
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
			if err = keystore.InstallKey(k, md.ToPath, t.Perm, t.DirPerm, t.User, t.Group); err != nil {
				return changed, err
			}
			changed = true
		}
	}
	return changed, nil
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
		perm = *mode
	}
	if !file.ExistsAndDir(p) {
		if err := os.MkdirAll(p, perm); err != nil {
			return err
		}
	} else if file.Exists(p) {
		return fmt.Errorf("directory path %s is already occupied by a non-directory", p)
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
