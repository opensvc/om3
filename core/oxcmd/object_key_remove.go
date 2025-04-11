package oxcmd

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdObjectKeyRemove struct {
		OptsGlobal
		Name string
	}
)

func (t *CmdObjectKeyRemove) Run(selector, kind string) error {
	ctx := context.Background()
	c, err := client.New()
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
		if !slices.Contains(naming.KindDataStore, path.Kind) {
			continue
		}
		if err := t.RunForPath(ctx, c, path); err != nil {
			return err
		}
	}
	return nil
}

func (t *CmdObjectKeyRemove) RunForPath(ctx context.Context, c *client.T, path naming.Path) error {
	params := api.DeleteObjectDataKeyParams{
		Name: t.Name,
	}
	response, err := c.DeleteObjectDataKeyWithResponse(ctx, path.Namespace, path.Kind, path.Name, &params)
	if err != nil {
		return err
	}
	switch response.StatusCode() {
	case http.StatusNoContent:
		return nil
	case http.StatusBadRequest:
		return fmt.Errorf("%s: %s", path, *response.JSON400)
	case http.StatusUnauthorized:
		return fmt.Errorf("%s: %s", path, *response.JSON401)
	case http.StatusForbidden:
		return fmt.Errorf("%s: %s", path, *response.JSON403)
	case http.StatusInternalServerError:
		return fmt.Errorf("%s: %s", path, *response.JSON500)
	default:
		return fmt.Errorf("%s: unexpected response: %s", path, response.Status())
	}
}
