package oxcmd

import (
	"context"
	"fmt"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/nodeaction"
	"github.com/opensvc/om3/v3/daemon/api"
	"github.com/opensvc/om3/v3/util/xsession"
)

type (
	CmdNodeRegister struct {
		OptsGlobal
		CredentialFile string
		User           string
		Password       string
		App            string
		NodeSelector   string
	}
)

func (t *CmdNodeRegister) Run() error {
	user, password, err := commoncmd.CollectorCredential(t.CredentialFile)
	if err != nil {
		return err
	}
	if user == "" && t.User != "" {
		// The deprecated --user and --password, kept for the commands
		// written against the previous release. A credential wins over
		// them: an operator naming one is asking for it to be used.
		user, password = t.User, t.Password
	}
	return nodeaction.New(
		nodeaction.WithRemoteNodes(t.NodeSelector),
		nodeaction.WithRemoteFunc(func(ctx context.Context, nodename string) (interface{}, error) {
			c, err := client.New()
			if err != nil {
				return nil, err
			}
			params := api.PostNodeActionRegisterParams{}
			{
				sessionID := xsession.SessionID().UUID()
				params.SessionID = &sessionID
			}
			body := api.PostNodeActionRegisterRequest{}
			if user != "" {
				body.User = &user
				body.Password = &password
			}
			if t.App != "" {
				body.App = &t.App
			}
			response, err := c.PostNodeActionRegisterWithResponse(ctx, nodename, &params, body)
			if err != nil {
				return nil, err
			}
			switch {
			case response.JSON200 != nil:
				return *response.JSON200, nil
			case response.JSON400 != nil:
				return nil, fmt.Errorf("node %s: %s", nodename, *response.JSON400)
			case response.JSON401 != nil:
				return nil, fmt.Errorf("node %s: %s", nodename, *response.JSON401)
			case response.JSON403 != nil:
				return nil, fmt.Errorf("node %s: %s", nodename, *response.JSON403)
			case response.JSON500 != nil:
				return nil, fmt.Errorf("node %s: %s", nodename, *response.JSON500)
			default:
				return nil, fmt.Errorf("node %s: unexpected response: %s", nodename, response.Status())
			}
		}),
		nodeaction.WithFormat(t.Output),
		nodeaction.WithSort(t.Sort),
		nodeaction.WithColor(t.Color),
	).Do()
}
