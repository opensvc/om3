package commoncmd

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/datastore"
	"github.com/opensvc/om3/v3/util/uri"
)

// MakeDataPatch returns the key operation of action on the key name, valued
// value, or one operation per file read from the from location, named after
// the file.
func MakeDataPatch(name string, value, from *string, action api.PatchDataKeyAction) (api.PatchDataKeys, error) {
	data := make(api.PatchDataKeys, 0)
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

// RunObjectDataPatch applies the key operations of data to the datastores
// the selector names, through the api.
//
// With wait, each answer is held until the configuration the operations
// committed has reached every live node of the object, for no longer than
// duration.
func RunObjectDataPatch(selector string, data api.PatchDataKeys, wait bool, duration time.Duration) error {
	c, ctx, cancel, err := ConfigWaitClient(wait, duration)
	if err != nil {
		return err
	}
	defer cancel()
	paths, err := objectselector.New(selector, objectselector.WithClient(c)).Expand()
	if err != nil {
		return err
	}
	for _, path := range paths {
		if !slices.Contains(naming.KindDataStore, path.Kind) {
			continue
		}
		if err := PatchObjectData(ctx, c, path, data, wait, duration); err != nil {
			return err
		}
	}
	return nil
}

// PatchObjectData applies the key operations of data to the datastore p,
// through the api, waiting for the configuration to reach every live node of
// the object when wait is set.
func PatchObjectData(ctx context.Context, c *client.T, p naming.Path, data api.PatchDataKeys, wait bool, duration time.Duration) error {
	params := api.PatchObjectDataParams{}
	if wait {
		s := duration.String()
		params.Wait = &s
	}
	resp, err := c.PatchObjectDataWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params, data)
	if err != nil {
		return err
	}
	switch {
	case resp.StatusCode() == http.StatusNoContent:
		return nil
	case resp.JSON408 != nil:
		return fmt.Errorf("%s: %s", p, resp.JSON408.Detail)
	case resp.JSON409 != nil:
		return fmt.Errorf("%s: %s", p, resp.JSON409.Detail)
	case resp.StatusCode() == http.StatusNotFound:
		return fmt.Errorf("%s: not found", p)
	case resp.JSON400 != nil:
		return fmt.Errorf("%s: %s", p, resp.JSON400.Detail)
	case resp.JSON401 != nil:
		return fmt.Errorf("%s: %s", p, resp.JSON401.Detail)
	case resp.JSON403 != nil:
		return fmt.Errorf("%s: %s", p, resp.JSON403.Detail)
	case resp.JSON413 != nil:
		return fmt.Errorf("%s: %s", p, resp.JSON413.Detail)
	case resp.JSON500 != nil:
		return fmt.Errorf("%s: %s", p, resp.JSON500.Detail)
	default:
		return fmt.Errorf("%s: unexpected response: %s", p, resp.Status())
	}
}
