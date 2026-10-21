package oxcmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/object"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/output"
	"github.com/opensvc/om3/v3/core/rawconfig"
	"github.com/opensvc/om3/v3/core/xerrors"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdObjectCertificateRenew struct {
		OptsGlobal
		Force bool
	}
)

func (t *CmdObjectCertificateRenew) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	ctx := context.Background()
	// A renewal from an ACME directory answers once the directory issued
	// the certificate, longer than a client waits by default.
	c, err := client.New(client.WithTimeout(0))
	if err != nil {
		return err
	}
	paths, err := objectselector.New(mergedSelector, objectselector.WithClient(c)).MustExpand()
	if err != nil {
		if t.IgnoreNotFound && errors.Is(err, xerrors.ObjectNotFound) {
			return nil
		}
		return err
	}
	params := api.PostObjectCertificateRenewParams{}
	if t.Force {
		params.Force = &t.Force
	}
	var errs error
	results := make([]object.CertificateRenewal, 0, len(paths))
	for _, p := range paths {
		resp, err := c.PostObjectCertificateRenewWithResponse(ctx, p.Namespace, p.Kind, p.Name, &params)
		switch {
		case err != nil:
			errs = errors.Join(errs, fmt.Errorf("%s: %w", p, err))
		case resp.JSON200 == nil:
			errs = errors.Join(errs, fmt.Errorf("%s: %s", p, certificateProblem(resp.Status(), resp.Body)))
		default:
			result := object.CertificateRenewal{
				Renewed: resp.JSON200.Renewed,
				Reason:  resp.JSON200.Reason,
				Sec:     p.String(),
				Domains: resp.JSON200.Domains,
			}
			if resp.JSON200.Directory != nil {
				result.Directory = *resp.JSON200.Directory
			}
			if resp.JSON200.NotAfter != nil {
				result.NotAfter = *resp.JSON200.NotAfter
			}
			results = append(results, result)
		}
	}
	if len(results) == 0 {
		return errs
	}
	return errors.Join(errs, output.Renderer{
		Output: t.Output,
		Sort:   t.Sort,
		Color:  t.Color,
		Data:   results,
		HumanRenderer: func() string {
			l := make([]string, len(results))
			for i, result := range results {
				l[i] = result.Render()
			}
			return strings.Join(l, "\n")
		},
		Colorize: rawconfig.Colorize,
	}.Print())
}
