package omcmd

import (
	"context"
	"fmt"
	"os"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/core/commoncmd"
	"github.com/opensvc/om3/core/keywords"
	"github.com/opensvc/om3/core/naming"
	"github.com/opensvc/om3/daemon/api"
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

	items := make(api.KeywordDefinitionItems, 0)
	index := keywords.ParseIndex(t.Keyword)
	params := api.GetClusterConfigKeywordsParams{}
	if index[0] != "" {
		params.Section = &index[0]
	}
	if index[1] != "" {
		params.Option = &index[1]
	}
	if t.Driver != "" {
		params.Driver = &t.Driver
	}

	response, err := c.GetClusterConfigKeywordsWithResponse(context.Background(), &params)
	if err != nil {
		return err
	}
	switch {
	case response.JSON200 != nil:
		items = append(items, response.JSON200.Items...)
	case response.JSON400 != nil:
		return fmt.Errorf("%s", *response.JSON400)
	case response.JSON401 != nil:
		return fmt.Errorf("%s", *response.JSON401)
	case response.JSON500 != nil:
		return fmt.Errorf("%s", *response.JSON500)
	default:
		return fmt.Errorf("unexpected response: %s", response.Status())
	}

	return commoncmd.Doc(os.Stdout, items, naming.KindCcfg, t.Depth)
}
