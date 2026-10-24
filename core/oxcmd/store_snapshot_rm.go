package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdStoreSnapshotRm struct {
		OptsGlobal
		Store    string
		Snapshot string
	}
)

// Run removes the run the snapshot id names from the store, protected or
// not: the retention never removes a protected one.
func (t *CmdStoreSnapshotRm) Run() error {
	c, err := client.New()
	if err != nil {
		return err
	}
	params := api.DeleteStoreSnapshotParams{Store: t.Store, Id: t.Snapshot}
	resp, err := c.DeleteStoreSnapshotWithResponse(context.Background(), &params)
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case 204:
		fmt.Printf("store %s: removed the snapshot %s\n", t.Store, t.Snapshot)
		return nil
	case 400:
		return fmt.Errorf("%s", resp.JSON400)
	case 401:
		return fmt.Errorf("%s", resp.JSON401)
	case 403:
		return fmt.Errorf("%s", resp.JSON403)
	case 404:
		return fmt.Errorf("%s", resp.JSON404)
	case 500:
		return fmt.Errorf("%s", resp.JSON500)
	default:
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode())
	}
}
