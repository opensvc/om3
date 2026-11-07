package commoncmd

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

func NewCmdAnyConfigUpdate() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update",
		Short: "update configuration",
		Long: `Apply a batch of configuration changes as a single transaction.

Changes are applied to a buffer in a this order:
1/ section deletes given by --delete=<section>
2/ keyword unsets given by --unset=<section>.<option>
3/ keyword sets given by --set=<section>.<option><op><value>

Then validate the new configuration.
Finally commit if no error was found.

Valid operators are:
* = set exact value
* |= append value if not already in list
* += append value even if already in list
* -= remove value from the list

Validate the new configuration and commit.`,
	}
	return cmd
}

// FlagConfigWait is the --wait of a configuration update.
func FlagConfigWait(flags *pflag.FlagSet, p *bool) {
	flags.BoolVar(p, "wait", false, "wait for the configuration to reach every live node of the object")
}

// PatchObjectConfig writes the keyword operations of params to the
// configuration of p, and says whether it changed, and what the daemon
// warns of, the write being done.
//
// With params.Wait, the daemon holds the answer until the configuration has
// reached every live node of the object. A wait that expires is an error
// naming where it has not landed, after a write that was made all the same:
// writing again would apply a += operation twice.
func PatchObjectConfig(ctx context.Context, c *client.T, p naming.Path, params api.PatchObjectConfigParams) (bool, []string, error) {
	result, err := patchObjectConfig(ctx, c, p, params)
	if err != nil {
		return false, nil, err
	}
	committed, err := result.AsCommitted()
	if err != nil {
		return false, nil, fmt.Errorf("%s: %w", p, err)
	}
	var warnings []string
	if committed.Warnings != nil {
		warnings = *committed.Warnings
	}
	return committed.IsChanged, warnings, nil
}

// DryRunPatchObjectConfig has the daemon run every check of the keyword
// operations of params on the configuration of p, and write nothing. It
// returns the alerts the checks found: the problems the update would be
// refused for, and the warnings of the validation.
func DryRunPatchObjectConfig(ctx context.Context, c *client.T, p naming.Path, params api.PatchObjectConfigParams) (*api.ConfigAlertList, error) {
	dryRun := true
	params.DryRun = &dryRun
	params.Wait = nil
	result, err := patchObjectConfig(ctx, c, p, params)
	if err != nil {
		return nil, err
	}
	alerts, err := result.AsConfigAlertList()
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return &alerts, nil
}

func patchObjectConfig(ctx context.Context, c *client.T, p naming.Path, params api.PatchObjectConfigParams) (*api.ConfigUpdateResult, error) {
	resp, err := c.PatchObjectConfigWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case http.StatusOK:
		if resp.JSON200 == nil {
			return nil, fmt.Errorf("%s: unexpected response: %s", p, resp.Status())
		}
		return resp.JSON200, nil
	case http.StatusBadRequest:
		return nil, fmt.Errorf("%s: %s", p, *resp.JSON400)
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("%s: %s", p, *resp.JSON401)
	case http.StatusForbidden:
		return nil, fmt.Errorf("%s: %s", p, *resp.JSON403)
	case http.StatusNotFound:
		return nil, fmt.Errorf("%s: %s", p, *resp.JSON404)
	case http.StatusRequestTimeout:
		return nil, fmt.Errorf("%s: %s", p, resp.JSON408.Detail)
	case http.StatusConflict:
		return nil, fmt.Errorf("%s: %s", p, *resp.JSON409)
	case http.StatusInternalServerError:
		return nil, fmt.Errorf("%s: %s", p, *resp.JSON500)
	default:
		return nil, fmt.Errorf("%s: unexpected response: %s", p, resp.Status())
	}
}

// ConfigWaitClient returns the client a configuration update is made with,
// and the context bounding it.
//
// A wait holds the answer for as long as the wait, which is longer than a
// client allows an answer by default, so the client allows it and the
// context bounds it, with a margin for the answer to travel.
func ConfigWaitClient(wait bool, duration time.Duration) (*client.T, context.Context, context.CancelFunc, error) {
	if !wait {
		c, err := client.New()
		ctx, cancel := context.WithCancel(context.Background())
		return c, ctx, cancel, err
	}
	c, err := client.New(client.WithTimeout(0))
	ctx, cancel := context.WithTimeout(context.Background(), duration+5*time.Second)
	return c, ctx, cancel, err
}
