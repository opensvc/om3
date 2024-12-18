package omcmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

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

var (
	ErrCmdDaemonAuth = errors.New("command daemon auth")
)

func (t *CmdDaemonAuth) Run() error {
	if err := t.checkParams(); err != nil {
		return fmt.Errorf("%w: %w", ErrCmdDaemonAuth, err)
	}
	c, err := client.New()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCmdDaemonAuth, err)
	}
	duration := t.Duration.String()
	roles := make(api.Roles, 0)
	for _, s := range t.Roles {
		roles = append(roles, api.Role(s))
	}
	params := api.PostAuthTokenParams{
		Duration: &duration,
	}
	if len(roles) > 0 {
		// Don't set params.Role when --role isn't used
		params.Role = &roles
	}
	resp, err := c.PostAuthTokenWithResponse(context.Background(), &params)
	if err != nil {
		return fmt.Errorf("%w: %w: %w", ErrCmdDaemonAuth, ErrClientRequest, err)
	} else if resp.StatusCode() != http.StatusOK {
		return fmt.Errorf("%w: %w: got %d wanted %d", ErrCmdDaemonAuth, ErrClientStatusCode, resp.StatusCode(), http.StatusOK)
	}
	if len(t.Out) == 0 {
		t.Out = []string{"token", "expire_at"}
	}
	for _, out := range t.Out {
		switch out {
		case "token":
			if _, err := fmt.Printf("%s\n", resp.JSON200.Token); err != nil {
				return fmt.Errorf("%w: %w: token: %w", ErrCmdDaemonAuth, ErrPrint, err)
			}
		case "expired_at":
			if _, err := fmt.Printf("%s\n", resp.JSON200.ExpiredAt); err != nil {
				return fmt.Errorf("%w: %w: expired_at: %w", ErrCmdDaemonAuth, ErrPrint, err)
			}
		}
	}
	return nil
}

func (t *CmdDaemonAuth) checkParams() error {
	if len(t.Out) == 0 {
		return fmt.Errorf("%w: out is empty", ErrFlagInvalid)
	}
	for _, s := range t.Out {
		switch s {
		case "token":
		case "expired_at":
		default:
			return fmt.Errorf("%w: out contains unexpected value: %s", ErrFlagInvalid, s)
		}
	}
	return nil
}
