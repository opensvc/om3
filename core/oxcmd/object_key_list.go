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
	CmdObjectKeyList struct {
		OptsGlobal
		Match string
	}
)

func (t *CmdObjectKeyList) Run(selector, kind string) error {
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
	result := api.DataStoreKeyList{
		Kind:  "DataStoreKeyList",
		Items: make(api.DataStoreKeyListItems, 0),
	}
	for _, path := range paths {
		if !slices.Contains(naming.KindDataStore, path.Kind) {
			continue
		}
		if moreKeys, err := t.RunForPath(ctx, c, path); err != nil {
			return err
		} else {
			result.Items = append(result.Items, moreKeys...)
		}
	}
	output.Renderer{
		DefaultOutput: "tab=OBJECT:object,NODE:node,NAME:name,SIZE:size",
		Output:        t.Output,
		Color:         t.Color,
		Data:          result,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return nil
}

func (t *CmdObjectKeyList) RunForPath(ctx context.Context, c *client.T, path naming.Path) (api.DataStoreKeyListItems, error) {
	response, err := c.GetObjectDataStoreKeysWithResponse(ctx, path.Namespace, path.Kind, path.Name)
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
