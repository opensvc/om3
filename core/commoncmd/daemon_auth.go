package commoncmd

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdDaemonAuth struct {
		Roles           []string
		Subject         string
		Scope           string
		AccessDuration  time.Duration
		Out             string
		Refresh         bool
		RefreshDuration time.Duration
	}
)

var (
	ErrCmdDaemonAuth = errors.New("command daemon auth")
)

func NewCmdDaemonAuth() *cobra.Command {
	var options CmdDaemonAuth
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "create new token",
		RunE: func(cmd *cobra.Command, args []string) error {
			return options.Run()
		},
	}
	flags := cmd.Flags()
	FlagRoles(flags, &options.Roles)
	flags.DurationVar(&options.AccessDuration, "duration", 60*time.Second, "access_token duration.")
	flags.DurationVar(&options.RefreshDuration, "refresh-duration", 24*time.Hour, "refresh_token duration.")
	flags.StringVarP(&options.Out, "output", "o", "auto", "output format json|flat|auto|tab=<header>:<jsonpath>,...")
	flags.StringVar(&options.Subject, "subject", "", "the subject of the token")
	flags.StringVar(&options.Scope, "scope", "", "the scope of the token grant")
	flags.BoolVar(&options.Refresh, "refresh", false, "also provide refresh token")
	return cmd
}

func (t *CmdDaemonAuth) Run() error {
	c, err := client.New()
	if err != nil {
		return fmt.Errorf("%w: %w", ErrCmdDaemonAuth, err)
	}
	duration := t.AccessDuration.String()
	refreshDuration := t.RefreshDuration.String()
	roles := make(api.Roles, 0)
	for _, s := range t.Roles {
		roles = append(roles, api.Role(s))
	}
	params := api.PostAuthTokenParams{
		AccessDuration: &duration,
		Subject:        &t.Subject,
		Scope:          &t.Scope,
	}
	if t.Refresh {
		params.Refresh = &t.Refresh
		params.RefreshDuration = &refreshDuration
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
	output.Renderer{
		DefaultOutput: "tab=:access_token",
		Output:        t.Out,
		Data:          *resp.JSON200,
		Colorize:      rawconfig.Colorize,
	}.Print()
	return nil
}
