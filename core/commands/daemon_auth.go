package commands

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/pkg/errors"

	"github.com/opensvc/om3/core/client"
	"github.com/opensvc/om3/daemon/api"
)

type (
	CmdDaemonAuth struct {
		OptsGlobal
		Roles    []string
		Duration time.Duration
		Out      []string
	}
)

func (t *CmdDaemonAuth) Run() error {
	c, err := client.New(
		client.WithURL(t.Server),
	)
	if err != nil {
		return err
	}
	duration := t.Duration.String()
	roles := make(api.QueryRoles, 0)
	for _, s := range t.Roles {
		roles = append(roles, api.Role(s))
	}
	params := api.PostAuthTokenParams{
		Duration: &duration,
		Role:     &roles,
	}
	resp, err := c.PostAuthTokenWithResponse(context.Background(), &params)
	if err != nil {
		return err
	} else if resp.StatusCode() != http.StatusOK {
		return errors.Errorf("unexpected post auth token status code %s", resp.Status())
	}
	if len(t.Out) == 0 {
		t.Out = []string{"token", "token_expire_at"}
	} else {
		for _, out := range t.Out {
			switch out {
			case "token":
				if _, err := fmt.Printf("%s\n", resp.JSON200.Token); err != nil {
					return err
				}
			case "token_expire_at":
				if _, err := fmt.Printf("%s\n", resp.JSON200.TokenExpireAt); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
