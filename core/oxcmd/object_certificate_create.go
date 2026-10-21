package oxcmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/opensvc/om3/v3/core/client"
	"github.com/opensvc/om3/v3/core/commoncmd"
	"github.com/opensvc/om3/v3/core/objectselector"
	"github.com/opensvc/om3/v3/core/xerrors"
	"github.com/opensvc/om3/v3/daemon/api"
)

type (
	CmdSecGenCert struct {
		OptsGlobal
	}
)

func (t *CmdSecGenCert) Run(kind string) error {
	mergedSelector := commoncmd.MergeSelector("", t.ObjectSelector, kind, "")
	ctx := context.Background()
	c, err := client.New()
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
	var errs error
	for _, p := range paths {
		resp, err := c.PostObjectCertificateCreateWithResponse(ctx, p.Namespace, p.Kind, p.Name)
		switch {
		case err != nil:
			errs = errors.Join(errs, fmt.Errorf("%s: %w", p, err))
		case resp.StatusCode() != http.StatusNoContent:
			errs = errors.Join(errs, fmt.Errorf("%s: %s", p, certificateProblem(resp.Status(), resp.Body)))
		}
	}
	return errs
}

// certificateProblem returns the problem a certificate request answered, as
// the daemon says it.
func certificateProblem(status string, body []byte) string {
	var problem api.Problem
	if err := json.Unmarshal(body, &problem); err != nil || problem.Title == "" {
		return fmt.Sprintf("unexpected response: %s", status)
	}
	return problem.String()
}
