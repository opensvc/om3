package oxcmd

import (
	"context"
	"fmt"
	"slices"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdStoreSnapshotList struct {
		OptsGlobal
		Store   string
		Path    string
		Cluster string
	}
)

func (t *CmdStoreSnapshotList) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.GetStoreSnapshotsParams{}
	if t.Store != "" {
		params.Store = &t.Store
	}
	if t.Path != "" {
		params.Path = &t.Path
	}
	if t.Cluster != "" {
		params.Cluster = &t.Cluster
	}
	resp, err := c.GetStoreSnapshotsWithResponse(context.Background(), &params)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 200:
		// The api answers newest first, for a page to show the latest:
		// listed whole, oldest first, as om lists them.
		slices.Reverse(resp.JSON200.Items)
		commoncmd.LocalStoreSnapshots(resp.JSON200.Items)
		return output.Renderer{
			DefaultOutput: "tab=STORE:store,PATH:path,CREATED_AT:created_at,SNAPSHOT:id,CLUSTER:cluster_name,NODE:node,MEMBERS:members[*],PROTECTED:protected,NOTE:note",
			Output:        t.Output,
			Sort:          t.Sort,
			Color:         t.Color,
			Data:          resp.JSON200.Items,
			Colorize:      rawconfig.Colorize,
		}.Print()
	case 400:
		return fmt.Errorf("%s", resp.JSON400)
	case 401:
		return fmt.Errorf("%s", resp.JSON401)
	case 403:
		return fmt.Errorf("%s", resp.JSON403)
	case 500:
		return fmt.Errorf("%s", resp.JSON500)
	default:
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}
}
