package commoncmd

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/spf13/pflag"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/naming"
	"github.com/opensvc/om3/v3/daemon/api"
)

// FlagDraftConfig adds the --config of a validation: a draft configuration
// to validate, rather than the one the object holds.
func FlagDraftConfig(flags *pflag.FlagSet, p *string) {
	flags.StringVar(p, "config", "", "validate this configuration, a file path or - for stdin, as the daemon checks a write of it to the object, and write nothing")
}

// ValidateDraftConfig has the daemon validate the configuration read from
// src, a file path or - for stdin, as a write of it to the object of
// selector, a single path, would be checked: a create of an object that does
// not exist, a replace of one that does. The alerts are printed, and the
// error says whether one is an error.
func ValidateDraftConfig(ctx context.Context, selector, src, out, sort, color string) error {
	p, err := naming.ParsePath(selector)
	if err != nil {
		return fmt.Errorf("--config validates the configuration of one object, named by its path: %w", err)
	}
	var b []byte
	switch src {
	case "-", "/dev/stdin", "stdin":
		b, err = DataFromStdin()
	default:
		b, err = DataFromConfigFile(src)
	}
	if err != nil {
		return err
	}
	c, err := client.New()
	if err != nil {
		return err
	}
	resp, err := c.PostObjectConfigValidateWithBodyWithResponse(ctx, p.Namespace, p.Kind, p.Name, "application/octet-stream", bytes.NewReader(b))
	if err != nil {
		return err
	}
	switch resp.StatusCode() {
	case http.StatusOK:
	case http.StatusBadRequest:
		return fmt.Errorf("%s: %s", p, *resp.JSON400)
	case http.StatusUnauthorized:
		return fmt.Errorf("%s: %s", p, *resp.JSON401)
	case http.StatusForbidden:
		return fmt.Errorf("%s: %s", p, *resp.JSON403)
	case http.StatusInternalServerError:
		return fmt.Errorf("%s: %s", p, *resp.JSON500)
	default:
		return fmt.Errorf("%s: unexpected response: %s", p, resp.Status())
	}
	items := api.ConfigAlertItems{}
	if resp.JSON200 != nil && resp.JSON200.Items != nil {
		items = resp.JSON200.Items
	}
	if err := PrintConfigAlerts(items, out, sort, color); err != nil {
		return err
	}
	return ConfigAlertErrors(items)
}
