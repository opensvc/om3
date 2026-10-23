package oxcmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/util/file"
	"github.com/opensvc/om3/v3/util/uri"
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
		Adopt       bool
		Force       bool
		Namespace   string

		// Store and Snapshot name a snapshot to create the object from:
		// the daemon reads it, and names it for the provision to restore
		// the heads from.
		Store    string
		Snapshot string

		client *client.T
		path   naming.Path
	}
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
		// The stream has a client of its own, with no timeout: a client
		// timeout bounds the whole answer, the body read included, and
		// would cut the stream after it. The context bounds it instead.
		streamClient, err := client.New(client.WithTimeout(0))
		if err != nil {
			return err
		}
		if err := commoncmd.WaitAllInstanceMonitor(ctx, streamClient, t.path, 0, errC); err != nil {
			// Wait until all instance monitors are registered before continuing, otherwise the next orchestration
			// step may return early due to missing cluster monitors.
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
			// A restored object finds what it holds, as replicated storage,
			// and takes it over.
			Adopt: t.Adopt || t.Restore,
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
	if t.Store != "" || t.Snapshot != "" {
		return t.fromStore()
	}
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
	case pathsCount > 0:
		return fmt.Errorf("can't create from multiple existing object: %s", paths)
	default:
		return t.fromConfig()
	}
}

// fromData creates the object at p from the configuration b, through the
// api, which checks it as any configuration it writes, and leaves behind
// what a copied configuration recorded of itself, unless --restore, the
// keyword operations applying after.
func (t *CmdObjectCreate) fromData(p naming.Path, b []byte) error {
	return commoncmd.CreateObjectConfig(context.Background(), t.client, p, b, t.Keywords, t.Restore, t.Force)
}

// fromStore creates the object from a snapshot: the configuration of the
// object the snapshot is of, which the daemon reads from the store.
func (t *CmdObjectCreate) fromStore() error {
	switch {
	case t.Store == "":
		return fmt.Errorf("--snapshot needs --store, the store holding it")
	case t.Snapshot == "":
		return fmt.Errorf("--store needs --snapshot, an id or the path of an object whose latest snapshot to take")
	case t.Config != "":
		return fmt.Errorf("--store and --config are mutually exclusive: the configuration is the one of the snapshot")
	case t.path.IsZero():
		return fmt.Errorf("need a target object path")
	}
	return commoncmd.CreateObjectConfigFromStore(context.Background(), t.client, t.path, t.Store, t.Snapshot, t.Keywords, t.Restore, t.Force)
}

func (t *CmdObjectCreate) fromPath(p naming.Path) error {
	cmd := CmdObjectConfigShow{}
	b, err := cmd.extractFromDaemon(p, t.client)
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

func (t *CmdObjectCreate) fromTemplate(template string) error {
	if b, err := commoncmd.DataFromTemplate(template); err != nil {
		return err
	} else {
		return t.fromData(t.path, b)
	}
}

func (t *CmdObjectCreate) fromConfig() error {
	b, err := t.dataFromConfig()
	if err != nil {
		return err
	} else {
		return t.fromData(t.path, b)
	}
}

func (t *CmdObjectCreate) fromScratch() error {
	return t.fromData(t.path, nil)
}

func (t *CmdObjectCreate) fromStdin() error {
	b, err := commoncmd.DataFromStdin()
	if err != nil {
		return err
	}
	return t.fromData(t.path, b)
}

func (t *CmdObjectCreate) dataFromConfig() ([]byte, error) {
	u := uri.New(t.Config)
	switch {
	case file.Exists(t.Config):
		return commoncmd.DataFromConfigFile(t.Config)
	case u.IsValidHttp():
		return commoncmd.DataFromConfigHttp(u)
	default:
		return nil, fmt.Errorf("invalid configuration: %s is not a file, nor an uri", t.Config)
	}
}
