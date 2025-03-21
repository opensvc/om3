package omcmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/iancoleman/orderedmap"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/cmd"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/freeze"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/uri"
)

type (
	CmdObjectCreate struct {
		OptsGlobal
		commoncmd.OptsAsync
		commoncmd.OptsLock
		Config    string
		Keywords  []string
		Env       []string
		Provision bool
		Restore   bool
		Force     bool
		Namespace string

		client *client.T
		path   naming.Path
	}
	Pivot map[string]rawconfig.T
)

var (
	schemeTemplate string = "template://"
	schemeFile     string = "file://"
	schemeObject   string = "object://"
)

func (t *CmdObjectCreate) Run(selector, kind string) error {
	for _, e := range t.Env {
		t.Keywords = append(t.Keywords, "env."+e)
	}
	if p, err := t.parseSelector(selector, kind); err != nil {
		return err
	} else {
		t.path = p
	}
	if c, err := client.New(); err != nil {
		return err
	} else {
		t.client = c
	}
	errC := make(chan error)
	if t.Provision {
		if err := cmd.WaitInstanceMonitor(context.Background(), t.client, t.path, t.Time, errC); err != nil {
			return err
		}
	}
	if err := t.do(); err != nil {
		return err
	}
	if t.Provision {
		err := <-errC
		if err != nil {
			return err
		}
		provisionOptions := CmdObjectProvision{
			OptsGlobal: t.OptsGlobal,
			OptsAsync:  t.OptsAsync,
			OptsLock:   t.OptsLock,
		}
		if err := provisionOptions.Run(selector, kind); err != nil {
			return err
		}
	}
	return nil
}

func (t *CmdObjectCreate) parseSelector(selector, kind string) (naming.Path, error) {
	var objectPath string
	if selector != "" && t.ObjectSelector != "" {
		return naming.Path{}, fmt.Errorf("use either 'om <path> create' or 'om <kind> create -s <path>', not 'om <path> create -s <path>'")
	} else if selector == "" && t.ObjectSelector != "" {
		objectPath = t.ObjectSelector
	} else {
		objectPath = selector
	}
	if objectPath == "" {
		// allowed with multi-definitions fed via stdin
		return naming.Path{}, nil
	}
	p, err := naming.ParsePath(objectPath)
	if err != nil {
		return p, err
	}
	// now we know the path is valid. Verify it is non-existing or matches only one object.
	objectSelector := mergeSelector(objectPath, "", kind, "**")
	paths, err := objectselector.New(
		objectSelector,
		objectselector.WithLocal(t.Local),
		objectselector.WithClient(t.client),
	).Expand()
	if err == nil && len(paths) > 1 {
		return p, fmt.Errorf("at most one object can be selected for create. to create many objects in a single create, use --config - and pipe json definitions")
	}
	return p, nil
}

func (t *CmdObjectCreate) getTemplate() string {
	if strings.HasPrefix(t.Config, schemeTemplate) {
		return t.Config[len(schemeTemplate):]
	}
	if _, err := strconv.Atoi(t.Config); err == nil {
		return t.Config
	}
	return ""
}

func (t *CmdObjectCreate) getSourcePaths() naming.Paths {
	paths, _ := objectselector.New(
		t.Config,
		objectselector.WithLocal(t.Local),
		objectselector.WithClient(t.client),
	).Expand()
	return paths
}

func (t *CmdObjectCreate) do() error {
	template := t.getTemplate()
	paths := t.getSourcePaths()
	switch {
	case t.Config == "":
		return t.fromScratch()
	case t.Config == "-" || t.Config == "/dev/stdin" || t.Config == "stdin":
		return t.fromStdin()
	case template != "":
		return t.fromTemplate(template)
	case len(paths) > 0:
		return t.fromPaths(paths)
	default:
		return t.fromConfig()
	}
}

func (t *CmdObjectCreate) configFromRaw(p naming.Path, c rawconfig.T) (string, error) {
	o, err := object.New(p, object.WithVolatile(true))
	if err != nil {
		return "", err
	}
	oc := o.(object.Configurer)
	if err := oc.Config().LoadRaw(c); err != nil {
		return "", err
	}

	ops := keyop.ParseOps(t.Keywords)
	if !t.Restore {
		op := keyop.Parse("id=" + uuid.New().String())
		if op == nil {
			return "", fmt.Errorf("invalid id reset op")
		}
		ops = append(ops, *op)
	}

	if err := oc.Config().Set(ops...); err != nil {
		return "", err
	}
	return oc.Config().Raw().String(), nil
}

func (t CmdObjectCreate) fromPaths(paths naming.Paths) error {
	pivot := make(Pivot)
	multi := len(paths) > 1
	for _, p := range paths {
		obj, err := object.NewConfigurer(p, object.WithVolatile(true))
		if err != nil {
			return err
		}
		if multi {
			if t.Namespace != "" {
				p.Namespace = t.Namespace
			} else {
				return fmt.Errorf("can not create multiple objects without a target namespace")
			}
		} else {
			if t.path.IsZero() {
				return fmt.Errorf("need a target object path")
			}
			p = t.path
			if t.Namespace != "" {
				p.Namespace = t.Namespace
			}
		}
		pivot[p.String()] = obj.Config().Raw()
	}
	return t.fromData(pivot)
}

func (t CmdObjectCreate) fromTemplate(template string) error {
	if pivot, err := t.rawFromTemplate(template); err != nil {
		return err
	} else {
		return t.fromData(pivot)
	}
}

func (t CmdObjectCreate) fromConfig() error {
	if pivot, err := t.rawFromConfig(); err != nil {
		return err
	} else {
		return t.fromData(pivot)
	}
}

func (t CmdObjectCreate) fromScratch() error {
	if pivot, err := rawFromScratch(t.path); err != nil {
		return err
	} else {
		return t.fromData(pivot)
	}
}

func (t CmdObjectCreate) fromStdin() error {
	var (
		pivot Pivot
		err   error
	)
	if t.path.IsZero() {
		pivot, err = rawFromStdinNested(t.Namespace)
	} else {
		pivot, err = rawFromStdinFlat(t.path)
	}
	if err != nil {
		return err
	} else {
		return t.fromData(pivot)
	}
}

func (t CmdObjectCreate) fromData(pivot Pivot) error {
	return t.localFromData(pivot)
}

func (t CmdObjectCreate) rawFromTemplate(template string) (Pivot, error) {
	return nil, fmt.Errorf("todo: collector requester")
}

func (t CmdObjectCreate) rawFromConfig() (Pivot, error) {
	u := uri.New(t.Config)
	switch {
	case file.Exists(t.Config):
		return rawFromConfigFile(t.path, t.Config)
	case u.IsValid():
		return rawFromConfigURI(t.path, u)
	default:
		return nil, fmt.Errorf("invalid configuration: %s is not a file, nor an uri", t.Config)
	}
}

func rawFromConfigURI(p naming.Path, u uri.T) (Pivot, error) {
	fpath, err := u.Fetch()
	if err != nil {
		return make(Pivot), nil
	}
	defer os.Remove(fpath)
	return rawFromConfigFile(p, fpath)
}

func rawFromConfigFile(p naming.Path, fpath string) (Pivot, error) {
	pivot := make(Pivot)
	c, err := xconfig.NewObject("", fpath)
	if err != nil {
		return pivot, err
	}
	pivot[p.String()] = c.Raw()
	return pivot, nil
}

func rawFromScratch(p naming.Path) (Pivot, error) {
	pivot := make(Pivot)
	pivot[p.String()] = rawconfig.T{}
	return pivot, nil
}

func rawFromStdinNested(namespace string) (Pivot, error) {
	pivot := make(Pivot)
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return pivot, err
	}
	if err = json.Unmarshal(b, &pivot); err != nil {
		return pivot, err
	}
	if md, ok := pivot["metadata"]; ok {
		p, err := pathFromMetadata(md.Data)
		if err != nil {
			return pivot, err
		}
		if namespace != "" {
			p.Namespace = namespace
		}
		return rawFromBytesFlat(p, b)
	}
	return pivot, nil
}

func pathFromMetadata(data *orderedmap.OrderedMap) (naming.Path, error) {
	var name, namespace, kind string
	if s, ok := data.Get("name"); ok {
		if name, ok = s.(string); !ok {
			return naming.Path{}, fmt.Errorf("metadata format error: name")
		}
	}
	if s, ok := data.Get("kind"); ok {
		if kind, ok = s.(string); !ok {
			return naming.Path{}, fmt.Errorf("metadata format error: kind")
		}
	}
	if s, ok := data.Get("namespace"); ok {
		switch k := s.(type) {
		case nil:
			namespace = ""
		case string:
			namespace = k
		default:
			return naming.Path{}, fmt.Errorf("metadata format error: namespace")
		}
	}
	return naming.NewPathFromStrings(namespace, kind, name)
}

func rawFromStdinFlat(p naming.Path) (Pivot, error) {
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, err
	}
	return rawFromBytesFlat(p, b)
}

func rawFromBytesFlat(p naming.Path, b []byte) (Pivot, error) {
	pivot := make(Pivot)
	c := &rawconfig.T{}
	if err := json.Unmarshal(b, c); err != nil {
		return pivot, err
	}
	pivot[p.String()] = *c
	return pivot, nil
}

func (t CmdObjectCreate) localFromData(pivot Pivot) error {
	for opath, c := range pivot {
		p, err := naming.ParsePath(opath)
		if err != nil {
			return err
		}
		if err = t.localFromRaw(p, c); err != nil {
			return err
		}
	}
	return nil
}

func (t CmdObjectCreate) localFromRaw(p naming.Path, c rawconfig.T) error {
	if !t.Force && p.Exists() {
		return fmt.Errorf("%s already exists", p)
	}
	o, err := object.New(p)
	if err != nil {
		return err
	}
	oc := o.(object.Configurer)
	if err := oc.Config().LoadRaw(c); err != nil {
		return err
	}

	ops := keyop.ParseOps(t.Keywords)
	if !t.Restore {
		op := keyop.Parse("id=" + uuid.New().String())
		if op == nil {
			return fmt.Errorf("invalid id reset op")
		}
		ops = append(ops, *op)
	}

	if err := oc.Config().Set(ops...); err != nil {
		return err
	}

	// Freeze if orchestrate==ha and freeze capable, so the daemon
	// doesn't decide to start the instance too soon.
	orchestrate := oc.Config().GetString(key.Parse("orchestrate"))
	if orchestrate == "ha" {
		if err := freeze.Freeze(t.path.FrozenFile()); err != nil {
			return err
		}
	}

	return nil
}

func (t CmdObjectCreate) localEmpty(p naming.Path) error {
	if !t.Force && p.Exists() {
		return fmt.Errorf("%s already exists", p)
	}
	o, err := object.New(p)
	if err != nil {
		return err
	}
	oc := o.(object.Configurer)

	// empty any existing config
	c := rawconfig.New()
	if err := oc.Config().LoadRaw(c); err != nil {
		return err
	}
	if err := oc.Config().Set(keyop.ParseOps(t.Keywords)...); err != nil {
		return err
	}
	return oc.Config().Commit()
}
