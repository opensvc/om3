package oxcmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/keyop"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/object"
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
		Force       bool
		Namespace   string

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
		if err := commoncmd.WaitAllInstanceMonitor(ctx, t.client, t.path, 0, errC); err != nil {
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

func (t CmdObjectCreate) fromData(p naming.Path, b []byte) error {
	if !t.Force && p.Exists() {
		return fmt.Errorf("%s already exists", p)
	}
	tempFile, err := os.CreateTemp("", "")
	if err != nil {
		return err
	}
	tempName := tempFile.Name()
	tempFile.Close()
	oc, err := object.NewConfigurer(p, object.WithConfigFile(tempName), object.WithConfigData(b))
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
	b, err = commoncmd.DataFromConfigFile(tempName)
	if err != nil {
		return err
	}
	resp, err := t.client.PostObjectConfigFileWithBodyWithResponse(context.Background(), p.Namespace, p.Kind, p.Name, "application/octet-stream", bytes.NewBuffer(b))
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 204:
		fmt.Printf("%s: created\n", p)
	case 400:
		fmt.Printf("%s: %s\n", p, *resp.JSON400)
	default:
		return fmt.Errorf("%s: %s", p, resp.Status())
	}
	return nil
}

func (t CmdObjectCreate) fromPath(p naming.Path) error {
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
	} else {
		return t.fromData(t.path, b)
	}
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
