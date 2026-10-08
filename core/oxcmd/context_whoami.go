package oxcmd

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/env"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
)

type (
	CmdContextWhoAmI struct {
		commoncmd.OptsGlobal
		Context string
	}
)

// Run asks the cluster of the context who the user of its credentials is,
// and what they are granted.
func (t *CmdContextWhoAmI) Run(cmd *cobra.Command) error {
	if cmd.Flag("context").Changed {
		os.Setenv(env.ContextVar, t.Context)
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	resp, err := c.GetAuthWhoAmIWithResponse(context.Background())
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case http.StatusOK:
	case http.StatusUnauthorized:
		return fmt.Errorf("%s", *resp.JSON401)
	default:
		return fmt.Errorf("unexpected statuscode: %s", resp.Status())
	}
	return output.Renderer{
		DefaultOutput: "tab=NAME:name,NAMESPACE:namespace,AUTH:auth,GRANT:raw_grant",
		Output:        t.Output,
		Sort:          t.Sort,
		Color:         t.Color,
		Data:          *resp.JSON200,
		Colorize:      rawconfig.Colorize,
	}.Print()
}
