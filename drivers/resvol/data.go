package resvol

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

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

	// MetadataFilter is used in functions having common code for configs and secrets management.
	MetadataFilter int

	// SigRoute is a relation between a signal number and the id of a resource supporting signaling
	SigRoute struct {
		Signum syscall.Signal
		RID    string
	}
)

const (
	// MDSec is the secrets Metadata filter. Secrets in Usr-kind object is also filtered-in.
	MDSec MetadataFilter = 1

	// MDCfg is the configs Metadata filter
	MDCfg MetadataFilter = 2
)

func (t Metadata) IsEmpty() bool {
	return t.ToPath == "" && t.FromKey == ""
}

func (t T) getRefs(filter MetadataFilter) []string {
	refs := make([]string, 0)
	refs = append(refs, t.getFilteredRefs(MDSec)...)
	refs = append(refs, t.getFilteredRefs(MDCfg)...)
	return refs
}

func (t T) getFilteredRefs(filter MetadataFilter) []string {
	refs := make([]string, 0)
	if t.Head() == "" {
		// not yet provisioned
		return refs
	}
	switch filter {
	case MDSec:
		refs = append(refs, t.Secrets...)
	case MDCfg:
		refs = append(refs, t.Configs...)
	}
	return refs
}

func (t T) getMetadata() []Metadata {
	l := make([]Metadata, 0)
	l = append(l, t.getFilteredMetadata(MDSec)...)
	l = append(l, t.getFilteredMetadata(MDCfg)...)
	return l
}

func (t T) getFilteredMetadata(filter MetadataFilter) []Metadata {
	l := make([]Metadata, 0)
	mnt := t.Head()
	if mnt == "" {
		return []Metadata{}
	}
	for _, ref := range t.getFilteredRefs(filter) {
		var kd kind.T
		switch filter {
		case MDSec:
			kd = kind.Sec
		default:
			kd = kind.Cfg
		}
		md := t.parseReference(ref, kd, mnt)
		if md.IsEmpty() {
			continue
		}
		l = append(l, md)
	}
	return l
}

func (t T) parseReference(s string, kd kind.T, mnt string) Metadata {
	// s = "sec/s1/k[12]:/here/"
	l := strings.SplitN(s, ":", 2)
	if len(l) != 2 {
		return Metadata{}
	}
	if mnt == "" {
		return Metadata{}
	}
	toPath := filepath.Join(mnt, l[1])
	// toPath = "/here"

	from := strings.TrimLeft(l[0], "/")
	// from = "sec/s1/k[12]"

	switch {
	case strings.HasPrefix(from, "usr/"):
		kd = kind.Usr
		from = from[4:]
		if kd == kind.Cfg {
			return Metadata{}
		}
	case strings.HasPrefix(from, "sec/"):
		kd = kind.Sec
		from = from[4:]
		if kd == kind.Cfg {
			return Metadata{}
		}
	case strings.HasPrefix(from, "cfg/"):
		kd = kind.Cfg
		from = from[4:]
		if kd == kind.Sec {
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
		o := object.NewFromPath(md.FromStore, object.WithVolatile(true))
		base, _ := o.(object.Baser)
		if !base.Exists() {
			t.StatusLog().Warn("store %s does not exist: key %s data can not be installed in the volume", md.FromStore, md.FromKey)
			continue
		}
		keystore := o.(object.Keystorer)
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
	if v, err := t.installSecrets(); err != nil {
		return err
	} else {
		changed = v || changed
	}
	if v, err := t.installConfigs(); err != nil {
		return err
	} else {
		changed = v || changed
	}
	if changed {
		return t.sendSignals()
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

func (t T) sendSignals() error {
	type signaler interface {
		SignalResource(string, syscall.Signal) error
	}
	i := object.NewFromPath(t.Path)
	o, ok := i.(signaler)
	if !ok {
		return fmt.Errorf("%s does not implement SignalResource()", t.Path)
	}
	for _, sd := range t.signalData() {
		if err := o.SignalResource(sd.RID, sd.Signum); err != nil {
			return err
		}
		t.Log().Info().Msgf("resource %s has been sent a signal %d", sd.RID, sd.Signum)
	}
	return nil
}

func (t T) installSecrets() (bool, error) {
	return t.installFilteredData(MDSec)
}

func (t T) installConfigs() (bool, error) {
	return t.installFilteredData(MDCfg)
}

func (t T) installFilteredData(filter MetadataFilter) (bool, error) {
	var (
		changed bool
		err     error
	)

	for _, md := range t.getFilteredMetadata(filter) {
		o := object.NewFromPath(md.FromStore, object.WithVolatile(true))
		base, _ := o.(object.Baser)
		if !base.Exists() {
			t.Log().Warn().Msgf("store %s does not exist: key %s data can not be installed in the volume", md.FromStore, md.FromKey)
			continue
		}
		keystore, _ := o.(object.Keystorer)
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
	head := t.Head()
	for _, dir := range t.Directories {
		if err := t.installDir(dir, head, t.DirPerm); err != nil {
			return err
		}
	}
	return nil
}
