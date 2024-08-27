package oxcmd

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/api"
	"github.com/opensvc/om3/util/uri"
)

type (
	CmdKeystoreChange struct {
		OptsGlobal
		OptsLock
		Key   string
		From  string
		Value string
	}
)

func (t *CmdKeystoreChange) Run(selector, kind string) error {
	var (
		r io.Reader
	)
	if t.Value != "" {
		r = bytes.NewBuffer([]byte(t.Value))
	} else if b, err := uri.ReadAllFrom(t.From); err != nil {
		return err
	} else {
		r = bytes.NewBuffer(b)
	}

	ctx := context.Background()
	c, err := client.New(client.WithURL(t.Server))
	if err != nil {
		return err
	}
	paths, err := objectselector.New(
		selector,
		objectselector.WithClient(c),
	).Expand()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := t.RunForPath(ctx, c, path, r); err != nil {
			return err
		}
	}
	return nil
}

func (t *CmdKeystoreChange) RunForPath(ctx context.Context, c *client.T, path naming.Path, r io.Reader) error {
	params := api.PutObjectKVStoreEntryParams{
		Key: t.Key,
	}
	response, err := c.PutObjectKVStoreEntryWithBodyWithResponse(ctx, path.Namespace, path.Kind, path.Name, &params, "application/octet-stream", r)
	if err != nil {
		return err
	}
	switch {
	case response.StatusCode() == http.StatusNoContent:
		return nil
	case response.StatusCode() == http.StatusNotFound:
		return fmt.Errorf("%s: key does not exists. consider using the 'add' action", path)
	case response.JSON400 != nil:
		return fmt.Errorf("%s: %s", path, *response.JSON400)
	case response.JSON401 != nil:
		return fmt.Errorf("%s: %s", path, *response.JSON401)
	case response.JSON403 != nil:
		return fmt.Errorf("%s: %s", path, *response.JSON403)
	case response.JSON500 != nil:
		return fmt.Errorf("%s: %s", path, *response.JSON500)
	default:
		return fmt.Errorf("%s: unexpected response: %s", path, response.Status())
	}
}
