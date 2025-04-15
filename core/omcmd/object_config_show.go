package omcmd

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/rawconfig"
)

type (
	CmdObjectConfigShow struct {
		Sections []string
	}
)

type result map[string]rawconfig.T

func (t *CmdObjectConfigShow) extract(selector string) ([]byte, error) {
	c, err := client.New()
	if err != nil {
		return nil, err
	}
	paths, err := objectselector.New(
		selector,
		objectselector.WithClient(c),
	).MustExpand()
	if err != nil {
		return nil, err
	}
	switch len(paths) {
	case 0:
		return nil, fmt.Errorf("no match")
	case 1:
	default:
		return nil, fmt.Errorf("more than one match: %s", paths)
	}

	p := paths[0]

	if b, err := t.extractFromDaemon(p, c); err == nil {
		return b, nil
	} else if p.Exists() {
		return os.ReadFile(p.ConfigFile())
	} else {
		return nil, fmt.Errorf("%w, and no local instance to read from", err)
	}
}

func (t *CmdObjectConfigShow) extractFromDaemon(p naming.Path, c *client.T) ([]byte, error) {
	resp, err := c.GetObjectConfigFileWithResponse(context.Background(), p.Namespace, p.Kind, p.Name)

	if err != nil {
		return nil, err
	} else if resp.StatusCode() != http.StatusOK {
		return nil, fmt.Errorf("get object config: %s", resp.Status())
	}
	return resp.Body, nil
}

func (t *CmdObjectConfigShow) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, "", kind, "")
	b, err := t.extract(mergedSelector)
	if err != nil {
		return err
	}
	b = commoncmd.Sections(b, t.Sections)
	b = commoncmd.ColorizeINI(b)
	_, err = os.Stdout.Write(b)
	return err
}
