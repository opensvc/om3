package commoncmd

import (
	"context"
	"fmt"
	"os"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/doc"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdClusterConfigDoc struct {
		Color   string
		Output  string
		Keyword string
		Driver  string
		Depth   int
	}
)

func (t *CmdClusterConfigDoc) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}

	items, err := doc.FindKeywords(t.Keyword, t.Driver != "", func(section, option *string) (api.KeywordDefinitionItems, error) {
		params := api.GetClusterConfigKeywordsParams{
			Section: section,
			Option:  option,
		}
		if t.Driver != "" {
			params.Driver = &t.Driver
		}
		response, err := c.GetClusterConfigKeywordsWithResponse(context.Background(), &params)
		if err != nil {
			return nil, err
		}
		switch {
		case response.JSON200 != nil:
			return response.JSON200.Items, nil
		case response.JSON400 != nil:
			return nil, fmt.Errorf("%s", *response.JSON400)
		case response.JSON401 != nil:
			return nil, fmt.Errorf("%s", *response.JSON401)
		case response.JSON500 != nil:
			return nil, fmt.Errorf("%s", *response.JSON500)
		default:
			return nil, fmt.Errorf("unexpected response: %s", response.Status())
		}
	})
	if err != nil {
		return err
	}

	return Doc(os.Stdout, items, naming.KindCcfg, t.Driver, t.Keyword, t.Depth)
}
