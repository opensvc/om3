package omcmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/freeze"
	"github.com/opensvc/om3/core/keyop"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/object"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/util/file"
	"github.com/opensvc/om3/util/key"
	"github.com/opensvc/om3/util/uri"
)

type (
	CmdObjectCreate struct {
		OptsGlobal
		commoncmd.OptsAsync
		commoncmd.OptsLock
		Local     bool
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

func (t *CmdObjectCreate) Run(kind string) error {
	for _, e := range t.Env {
		t.Keywords = append(t.Keywords, "env."+e)
	}
	if p, err := t.parseSelector(kind); err != nil {
		return err
	} else {
		t.path = p
	}
	if t.path.IsZero() {
		return fmt.Errorf("the path of the new object is required")
	}
	if c, err := client.New(); err != nil {
		return err
	} else {
		t.client = c
	}

	// errC must be buffered because of early return if an error occurs during t.do()
	errC := make(chan error, 1)

	ctx, cancel := context.WithTimeout(context.Background(), t.Time)
	defer cancel()
	var needWait bool
	if t.Wait || t.Provision {
		// need dedicated client that override default client timeout with t.Timeout (zero means no timeout).
		if c, err := client.New(client.WithTimeout(t.Timeout)); err != nil {
			return err
		} else if err := commoncmd.WaitAllInstanceMonitor(ctx, c, t.path, t.Time, errC); err != nil {
			// Wait until all instance monitors are registered before continuing, otherwise the next orchestration
			// step may return early due to missing cluster monitors.
			if errors.Is(err, os.ErrNotExist) {
				// the daemon is not running.
			} else {
				return err
			}
		} else {
			needWait = true
		}
	}

	if err := t.do(); err != nil {
		return err
	}

	if needWait {
		err := <-errC
		if err != nil {
			return err
		}
	}

	if t.Provision {
		provisionOptions := CmdObjectProvision{
			OptsGlobal: t.OptsGlobal,
			OptsAsync:  t.OptsAsync,
			OptsLock:   t.OptsLock,
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
	pathsCount := len(paths)
	switch {
	case t.Config == "":
		return t.fromScratch()
	case t.Config == "-" || t.Config == "/dev/stdin" || t.Config == "stdin":
		return t.fromStdin()
	case template != "":
		return t.fromTemplate(template)
	case pathsCount == 1:
		return t.fromPath(paths[0])
	case pathsCount > 1:
		return fmt.Errorf("can't create from multiple existing object: %s", paths)
	default:
		return t.fromConfig()
	}
}

func (t CmdObjectCreate) fromPath(p naming.Path) error {
	cmd := CmdObjectConfigShow{}
	b, err := cmd.extractPath(p, t.client)
	if err != nil {
		return err
	}
	if t.path.IsZero() {
		return fmt.Errorf("need a target object path")
	}
	p = t.path
	if t.Namespace != "" {
		p.Namespace = t.Namespace
	}
	return t.fromData(p, b)
}

func (t CmdObjectCreate) fromTemplate(template string) error {
	if b, err := commoncmd.DataFromTemplate(template); err != nil {
		return err
	} else {
		return t.fromData(t.path, b)
	}
}

func (t CmdObjectCreate) fromConfig() error {
	b, err := t.dataFromConfig()
	if err != nil {
		return err
	}
	return t.fromData(t.path, b)
}

func (t CmdObjectCreate) fromScratch() error {
	return t.fromData(t.path, nil)
}

func (t CmdObjectCreate) fromStdin() error {
	b, err := commoncmd.DataFromStdin()
	if err != nil {
		return err
	}
	return t.fromData(t.path, b)
}

func (t CmdObjectCreate) dataFromConfig() ([]byte, error) {
	u := uri.New(t.Config)
	switch {
	case file.Exists(t.Config):
		return commoncmd.DataFromConfigFile(t.Config)
	case u.IsValid():
		return commoncmd.DataFromConfigURI(u)
	default:
		return nil, fmt.Errorf("invalid configuration: %s is not a file, nor an uri", t.Config)
	}
}

func (t CmdObjectCreate) fromData(p naming.Path, b []byte) error {
	if !t.Force && p.Exists() {
		return fmt.Errorf("%s already exists", p)
	}
	oc, err := object.NewConfigurer(p, object.WithConfigData(b))
	if err != nil {
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
