package oxcmd

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdObjectKeyRename struct {
		OptsGlobal
		Name string
		To   string
	}
)

func (t *CmdObjectKeyRename) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")

	data := api.PatchObjectDataJSONRequestBody{
		api.PatchDataKey{
			Name:   t.Name,
			To:     &t.To,
			Action: api.Rename,
		},
	}

	ctx := context.Background()
	c, err := client.New()
	if err != nil {
		return err
	}
	paths, err := objectselector.New(
		mergedSelector,
		objectselector.WithClient(c),
	).Expand()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if !slices.Contains(naming.KindDataStore, path.Kind) {
			continue
		}
		if err := t.RunForPath(ctx, c, path, data); err != nil {
			return err
		}
	}
	return nil
}

func (t *CmdObjectKeyRename) RunForPath(ctx context.Context, c *client.T, path naming.Path, data api.PatchObjectDataJSONRequestBody) error {
	response, err := c.PatchObjectDataWithResponse(ctx, path.Namespace, path.Kind, path.Name, data)
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
