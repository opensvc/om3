package oxcmd

import (
	"bytes"
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
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/core/xconfig"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/uri"
)

type (
	CmdObjectCreate struct {
		OptsGlobal
		commoncmd.OptsAsync
		commoncmd.OptsLock
		Config      string
		Keywords    []string
		Env         []string
		Interactive bool
		Provision   bool
		Restore     bool
		Force       bool
		Namespace   string

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

func (t *CmdObjectCreate) Run(kind string) error {
	for _, e := range t.Env {
		t.Keywords = append(t.Keywords, "env."+e)
	}
	if c, err := client.New(); err != nil {
		return err
	} else {
		t.client = c
	}
	if p, err := t.parseSelector(kind); err != nil {
		return err
	} else {
		t.path = p
	}

	// errC must be buffered because of early return if an error occurs during t.do()
	errC := make(chan error, 1)

	if t.Wait || t.Provision {
		ctx, cancel := context.WithTimeout(context.Background(), t.Time)
		defer cancel()
		if err := commoncmd.WaitInstanceMonitor(ctx, t.client, t.path, 0, errC); err != nil {
			return err
		}
	}

	if err := t.do(); err != nil {
		return err
	}

	if t.Wait || t.Provision {
		err := <-errC
		if err != nil {
			return err
		}
	}

	if t.Provision {
		provisionOptions := CmdObjectProvision{
			OptsGlobal: t.OptsGlobal,
			OptsAsync:  t.OptsAsync,
		}
		if err := provisionOptions.Run(kind); err != nil {
			return err
		}
	}
	return nil
}

func (t *CmdObjectCreate) parseSelector(kind string) (naming.Path, error) {
	objectPath := t.ObjectSelector
	if objectPath == "" {
		// allowed with multi-definitions fed via stdin
		return naming.Path{}, nil
	}
	p, err := naming.ParsePath(objectPath)
	if err != nil {
		return p, err
	}
	// now we know the path is valid. Verify it is non-existing or matches only one object.
	objectSelector := commoncmd.MergeSelector(objectPath, "", kind, "**")
	paths, err := objectselector.New(
		objectSelector,
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
	f, err := os.CreateTemp("", ".create-*")
	if err != nil {
		return "", err
	}
	tempName := f.Name()
	defer f.Close()

	o, err := object.New(p, object.WithVolatile(true), object.WithConfigFile(tempName))
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

func (t *CmdObjectCreate) submit(pivot Pivot) error {
	for pathStr, c := range pivot {
		path, err := naming.ParsePath(pathStr)
		if err != nil {
			return fmt.Errorf("%s: %s", path, err)
		}
		s, err := t.configFromRaw(path, c)
		if err != nil {
			return fmt.Errorf("%s: %s", path, err)
		}
		body := bytes.NewBufferString(s)
		resp, err := t.client.PostObjectConfigFileWithBodyWithResponse(context.Background(), path.Namespace, path.Kind, path.Name, "application/octet-stream", body)
		if err != nil {
			return fmt.Errorf("%s: %s", path, err)
		}
		switch resp.StatusCode() {
		case 204:
			fmt.Printf("%s: created\n", path)
		case 400:
			fmt.Printf("%s: %s\n", path, *resp.JSON400)
		default:
			return fmt.Errorf("%s: %s", path, resp.Status())
		}
	}
	return nil
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
	return t.submit(pivot)
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
