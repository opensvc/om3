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
	"github.com/opensvc/om3/util/keystore"
	"github.com/opensvc/om3/util/uri"
)

type (
	CmdObjectKeyChange struct {
		OptsGlobal
		commoncmd.OptsLock
		Key   string
		From  *string
		Value *string
	}
)

func makeKVStorePatch(key string, value, from *string, action api.PatchKVStoreEntryAction) (api.PatchObjectKVStoreJSONRequestBody, error) {
	data := make(api.PatchObjectKVStoreJSONRequestBody, 0)

	if value != nil {
		data = append(data, api.PatchKVStoreEntry{
			Key:    key,
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
		k, err := keystore.FileToKey(path, key, *from)
		if err != nil {
			return nil, err
		}
		data = append(data, api.PatchKVStoreEntry{
			Key:    k,
			Bytes:  &b,
			Action: action,
		})
	}
	return data, nil
}

func (t *CmdObjectKeyChange) Run(selector, kind string) error {
	if t.Value == nil && t.From == nil {
		return fmt.Errorf("a value or value source mut be specified for a change action")
	}
	data, err := makeKVStorePatch(t.Key, t.Value, t.From, api.Change)
	if err != nil {
		return err
	}

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
		if !slices.Contains(naming.KindKVStore, path.Kind) {
			continue
		}
		if err := t.RunForPath(ctx, c, path, data); err != nil {
			return err
		}
	}
	return nil
}

func (t *CmdObjectKeyChange) RunForPath(ctx context.Context, c *client.T, path naming.Path, data api.PatchObjectKVStoreJSONRequestBody) error {
	response, err := c.PatchObjectKVStoreWithResponse(ctx, path.Namespace, path.Kind, path.Name, data)
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
