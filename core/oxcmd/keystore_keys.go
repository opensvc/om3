package oxcmd

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/core/output"
	"github.com/opensvc/om3/core/rawconfig"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdKeystoreKeys struct {
		OptsGlobal
		Match string
	}
)

func (t *CmdKeystoreKeys) Run(selector, kind string) error {
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
	result := api.KVStoreKeyList{
		Kind:  "KVStoreKeyList",
		Items: make(api.KVStoreKeyListItems, 0),
	}
	for _, path := range paths {
		if !slices.Contains(naming.KindKVStore, path.Kind) {
			continue
		}
		if moreKeys, err := t.RunForPath(ctx, c, path); err != nil {
			return err
		} else {
			result.Items = append(result.Items, moreKeys...)
		}
	}
	output.Renderer{
		DefaultOutput: "tab=OBJECT:object,NODE:node,KEY:key",
		Output:        t.Output,
		Color:         t.Color,
		Data:          result,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return nil
}

func (t *CmdKeystoreKeys) RunForPath(ctx context.Context, c *client.T, path naming.Path) (api.KVStoreKeyListItems, error) {
	response, err := c.GetObjectKVStoreKeysWithResponse(ctx, path.Namespace, path.Kind, path.Name)
	if err != nil {
		return nil, err
	}
	switch response.StatusCode() {
	case http.StatusOK:
		return response.JSON200.Items, nil
	case http.StatusBadRequest:
		return nil, fmt.Errorf("%s: %s", path, *response.JSON400)
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("%s: %s", path, *response.JSON401)
	case http.StatusForbidden:
		return nil, fmt.Errorf("%s: %s", path, *response.JSON403)
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("%s: %s", path, *response.JSON500)
	default:
		return nil, fmt.Errorf("%s: unexpected response: %s", path, response.Status())
	}
}
