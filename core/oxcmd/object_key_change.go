package oxcmd

import (
	"context"
	"fmt"
	"net/http"
	"slices"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/datastore"
	"github.com/opensvc/om3/v3/util/uri"
)

type (
	CmdObjectKeyChange struct {
		OptsGlobal
		commoncmd.OptsLock
		Name  string
		From  *string
		Value *string
	}
)

func makeDataPatch(name string, value, from *string, action api.PatchDataKeyAction) (api.PatchObjectDataJSONRequestBody, error) {
	data := make(api.PatchObjectDataJSONRequestBody, 0)

	if value != nil {
		data = append(data, api.PatchDataKey{
			Name:   name,
			String: value,
			Action: action,
		})
		return data, nil
	}
	m, err := uri.ReadAllFrom(*from)
	if err != nil {
		return data, err
	}
	for path, b := range m {
		k, err := datastore.FileToKey(path, name, *from)
		if err != nil {
			return nil, err
		}
		data = append(data, api.PatchDataKey{
			Name:   k,
			Bytes:  &b,
			Action: action,
		})
	}
	return data, nil
}

func (t *CmdObjectKeyChange) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	if t.Value == nil && t.From == nil {
		return fmt.Errorf("a value or value source mut be specified for a change action")
	}
	data, err := makeDataPatch(t.Name, t.Value, t.From, api.Change)
	if err != nil {
		return err
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

func (t *CmdObjectKeyChange) RunForPath(ctx context.Context, c *client.T, path naming.Path, data api.PatchObjectDataJSONRequestBody) error {
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
	case response.JSON413 != nil:
		return fmt.Errorf("%s: %s", path, *response.JSON413)
	case response.JSON500 != nil:
		return fmt.Errorf("%s: %s", path, *response.JSON500)
	default:
		return fmt.Errorf("%s: unexpected response: %s", path, response.Status())
	}
}
