package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/objectselector"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdObjectSet struct {
		OptsGlobal
		OptsLock
		KeywordOps []string
	}
)

func (t *CmdObjectSet) Run(selector, kind string) error {
	mergedSelector := mergeSelector(selector, t.ObjectSelector, kind, "")
	c, err := client.New()
	if err != nil {
		return err
	}
	sel := objectselector.NewSelection(mergedSelector, objectselector.SelectionWithClient(c))
	paths, err := sel.Expand()
	if err != nil {
		return err
	}
	for _, p := range paths {
		params := api.PostObjectConfigUpdateParams{}
		params.Set = &t.KeywordOps
		response, err := c.PostObjectConfigUpdateWithResponse(context.Background(), p.Namespace, p.Kind, p.Name, &params)
		if err != nil {
			return err
		}
		switch response.StatusCode() {
		case 200:
			return nil
		case 400:
			return fmt.Errorf("%s: %s", p, *response.JSON400)
		case 401:
			return fmt.Errorf("%s: %s", p, *response.JSON401)
		case 403:
			return fmt.Errorf("%s: %s", p, *response.JSON403)
		case 500:
			return fmt.Errorf("%s: %s", p, *response.JSON500)
		default:
			return fmt.Errorf("%s: unexpected response: %s", p, response.Status())
		}
	}
	return nil
}
